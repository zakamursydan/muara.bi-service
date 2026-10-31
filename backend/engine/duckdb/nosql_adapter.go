package duckdb

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	// DefaultMongoDocLimit bounds how many documents a single collection streams
	// into DuckDB. The guardrail row limit is 10000, so streaming more could only
	// ever be discarded, while the flattened JSON buffer is held in memory.
	DefaultMongoDocLimit = 10000

	// MaxMongoCollections caps how many collections one connection mounts. Beyond
	// this a session would silently see an incomplete catalogue, so the mount
	// fails and the operator has to scope the connection to a smaller database.
	MaxMongoCollections = 32

	defaultMongoTimeout = 30 * time.Second
)

// MongoAdapterConfig holds parameters to stream one MongoDB collection into DuckDB
type MongoAdapterConfig struct {
	URI            string
	Database       string
	Collection     string
	Filter         map[string]any
	Limit          int64
	TimeoutSeconds int
}

// MongoCollectionStream mounts a whole MongoDB database: each collection becomes
// its own temporary view, because a NoSQL source has no catalog/schema nesting
// for DuckDB to address.
type MongoCollectionStream struct {
	URI            string
	Database       string
	Alias          string
	TimeoutSeconds int
}

// StreamMongoToSession queries MongoDB, flattens documents, and registers them as
// an in-memory DuckDB temporary view.
func StreamMongoToSession(ctx context.Context, session Session, viewName string, cfg MongoAdapterConfig) error {
	client, err := connectMongo(ctx, cfg.URI, cfg.TimeoutSeconds)
	if err != nil {
		return err
	}
	defer disconnectMongo(client)

	return streamCollection(ctx, client, session, cfg.Database, cfg.Collection, viewName, cfg)
}

// StreamMongoCollections registers every collection of the database as a
// `<alias>_<collection>` temporary view and returns the view names it mounted.
// Views are named rather than attached because DuckDB has no MongoDB scanner:
// the flattened JSON is the only representation the planner can see.
func StreamMongoCollections(ctx context.Context, session Session, cfg MongoCollectionStream) ([]string, error) {
	if cfg.Alias == "" {
		return nil, fmt.Errorf("mongodb stream needs a connection alias")
	}
	if cfg.Database == "" {
		return nil, fmt.Errorf("mongodb stream needs a database name")
	}

	client, err := connectMongo(ctx, cfg.URI, cfg.TimeoutSeconds)
	if err != nil {
		return nil, err
	}
	defer disconnectMongo(client)

	mongoCtx, cancel := mongoContext(ctx, cfg.TimeoutSeconds)
	defer cancel()

	names, err := client.Database(cfg.Database).ListCollectionNames(mongoCtx, bson.D{})
	if err != nil {
		return nil, fmt.Errorf("failed to list mongodb collections in %s: %w", cfg.Database, err)
	}
	if len(names) > MaxMongoCollections {
		return nil, fmt.Errorf("mongodb database %s has %d collections, above the %d limit; point this connection at a narrower database",
			cfg.Database, len(names), MaxMongoCollections)
	}

	views := make([]string, 0, len(names))
	seen := make(map[string]string, len(names))
	for _, name := range names {
		view := MongoViewName(cfg.Alias, name)
		if clash, ok := seen[view]; ok {
			// Two collections collapsing to one identifier would let one silently
			// overwrite the other, so refuse the mount instead.
			return nil, fmt.Errorf("collections %q and %q both map to view %q; rename one or use a dedicated connection", clash, name, view)
		}
		seen[view] = name

		if err := streamCollection(ctx, client, session, cfg.Database, name, view, MongoAdapterConfig{
			URI:            cfg.URI,
			Database:       cfg.Database,
			Limit:          DefaultMongoDocLimit,
			TimeoutSeconds: cfg.TimeoutSeconds,
		}); err != nil {
			return nil, err
		}
		views = append(views, view)
	}

	return views, nil
}

var nonIdentifier = regexp.MustCompile(`[^A-Za-z0-9_]+`)

// MongoViewName flattens an alias plus collection name into one plain DuckDB
// identifier, e.g. alias "dwh-logs" and collection "sessions" -> dwh_logs_sessions.
func MongoViewName(alias, collection string) string {
	candidate := nonIdentifier.ReplaceAllString(alias+"_"+collection, "_")
	if candidate == "" || !isIdentifierStart(candidate[0]) {
		candidate = "v_" + candidate
	}
	return strings.Trim(candidate, "_")
}

func isIdentifierStart(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// PingMongo checks a mongodb:// URI with the driver itself. A plain TCP dial
// accepts a server that will reject the credentials, so only a ping answers the
// question the connection test is being asked.
func PingMongo(ctx context.Context, uri string, timeoutSeconds int) error {
	client, err := connectMongo(ctx, uri, timeoutSeconds)
	if err != nil {
		return err
	}
	defer disconnectMongo(client)

	pingCtx, cancel := mongoContext(ctx, timeoutSeconds)
	defer cancel()
	if err := client.Ping(pingCtx, nil); err != nil {
		return fmt.Errorf("mongodb ping failed: %w", err)
	}
	return nil
}

func connectMongo(ctx context.Context, uri string, timeoutSeconds int) (*mongo.Client, error) {
	if uri == "" {
		return nil, fmt.Errorf("mongodb connection needs a URI")
	}
	connectCtx, cancel := context.WithTimeout(ctx, mongoTimeout(timeoutSeconds))
	defer cancel()

	client, err := mongo.Connect(connectCtx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mongodb: %w", err)
	}
	return client, nil
}

func disconnectMongo(client *mongo.Client) {
	if client == nil {
		return
	}
	// Best effort: the session is being torn down anyway, and a disconnect timeout
	// must not mask the error that ended the query.
	_ = client.Disconnect(context.Background())
}

func mongoTimeout(timeoutSeconds int) time.Duration {
	if timeoutSeconds > 0 {
		return time.Duration(timeoutSeconds) * time.Second
	}
	return defaultMongoTimeout
}

func mongoContext(ctx context.Context, timeoutSeconds int) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, mongoTimeout(timeoutSeconds))
}

func streamCollection(
	ctx context.Context,
	client *mongo.Client,
	session Session,
	database string,
	collection string,
	viewName string,
	cfg MongoAdapterConfig,
) error {
	mongoCtx, cancel := mongoContext(ctx, cfg.TimeoutSeconds)
	defer cancel()

	coll := client.Database(database).Collection(collection)

	findOpts := options.Find()
	if cfg.Limit > 0 {
		findOpts.SetLimit(cfg.Limit)
	} else {
		findOpts.SetLimit(DefaultMongoDocLimit)
	}

	filter := bson.M{}
	for k, v := range cfg.Filter {
		filter[k] = v
	}

	cursor, err := coll.Find(mongoCtx, filter, findOpts)
	if err != nil {
		return fmt.Errorf("mongodb query on %s.%s failed: %w", database, collection, err)
	}
	defer cursor.Close(mongoCtx)

	// Initialised so a collection with no documents marshals as `[]`; a nil slice
	// would serialise to `null` and read_json_auto rejects that as a table.
	flattenedDocs := make([]map[string]any, 0)
	for cursor.Next(mongoCtx) {
		var rawDoc map[string]any
		if err := cursor.Decode(&rawDoc); err != nil {
			return fmt.Errorf("failed to decode mongodb doc: %w", err)
		}

		flattened := make(map[string]any)
		flattenMap("", rawDoc, flattened)
		flattenedDocs = append(flattenedDocs, flattened)
	}

	if err := cursor.Err(); err != nil {
		return fmt.Errorf("mongodb cursor error on %s.%s: %w", database, collection, err)
	}

	jsonBuffer, err := json.Marshal(flattenedDocs)
	if err != nil {
		return fmt.Errorf("failed to marshal mongodb documents: %w", err)
	}

	if err := session.RegisterJSONData(ctx, viewName, jsonBuffer); err != nil {
		return fmt.Errorf("failed to register mongodb view %s in duckdb: %w", viewName, err)
	}
	return nil
}

// flattenMap recursively flattens nested maps into dot/underscore keys
func flattenMap(prefix string, src map[string]any, dest map[string]any) {
	for k, v := range src {
		key := k
		if prefix != "" {
			key = prefix + "_" + k
		}

		switch val := v.(type) {
		case map[string]any:
			flattenMap(key, val, dest)
		case bson.M:
			flattenMap(key, map[string]any(val), dest)
		case []any, primitive.A:
			// Arrays become serialized JSON so a mounted view keeps one stable
			// VARCHAR column. Leaving them native would make read_json_auto infer a
			// LIST whose element type shifts with whatever the documents hold.
			arrJSON, err := json.Marshal(val)
			if err == nil {
				dest[key] = string(arrJSON)
			} else {
				dest[key] = fmt.Sprintf("%v", val)
			}
		case primitive.ObjectID:
			// Mongo decodes documents into these driver types, not plain Go ones.
			dest[key] = val.Hex()
		default:
			dest[key] = v
		}
	}
}
