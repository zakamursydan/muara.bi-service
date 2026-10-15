package duckdb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoAdapterConfig holds parameters to stream MongoDB collection into DuckDB
type MongoAdapterConfig struct {
	URI            string
	Database       string
	Collection     string
	Filter         map[string]any
	Limit          int64
	TimeoutSeconds int
}

// StreamMongoToSession queries MongoDB, flattens documents, and registers as an in-memory DuckDB temporary view
func StreamMongoToSession(ctx context.Context, session Session, viewName string, cfg MongoAdapterConfig) error {
	timeout := 30 * time.Second
	if cfg.TimeoutSeconds > 0 {
		timeout = time.Duration(cfg.TimeoutSeconds) * time.Second
	}
	mongoCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := mongo.Connect(mongoCtx, options.Client().ApplyURI(cfg.URI))
	if err != nil {
		return fmt.Errorf("failed to connect to mongodb: %w", err)
	}
	defer func() {
		_ = client.Disconnect(context.Background())
	}()

	coll := client.Database(cfg.Database).Collection(cfg.Collection)

	findOpts := options.Find()
	if cfg.Limit > 0 {
		findOpts.SetLimit(cfg.Limit)
	} else {
		findOpts.SetLimit(10000) // Default safety limit
	}

	filter := bson.M{}
	if len(cfg.Filter) > 0 {
		for k, v := range cfg.Filter {
			filter[k] = v
		}
	}

	cursor, err := coll.Find(mongoCtx, filter, findOpts)
	if err != nil {
		return fmt.Errorf("mongodb query failed: %w", err)
	}
	defer cursor.Close(mongoCtx)

	var flattenedDocs []map[string]any
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
		return fmt.Errorf("mongodb cursor error: %w", err)
	}

	// Serialize flattened documents to JSON
	jsonBuffer, err := json.Marshal(flattenedDocs)
	if err != nil {
		return fmt.Errorf("failed to marshal mongodb documents: %w", err)
	}

	// Register in DuckDB session
	if err := session.RegisterJSONData(ctx, viewName, jsonBuffer); err != nil {
		return fmt.Errorf("failed to register mongodb virtual view %s in duckdb: %w", viewName, err)
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
		case []any:
			// Convert arrays to serialized JSON strings for analytical column stability
			arrJSON, err := json.Marshal(val)
			if err == nil {
				dest[key] = string(arrJSON)
			} else {
				dest[key] = fmt.Sprintf("%v", val)
			}
		default:
			// Handle MongoDB ObjectID hex representation if present
			if stringer, ok := val.(fmt.Stringer); ok && strings.HasPrefix(key, "_id") {
				dest[key] = stringer.String()
			} else {
				dest[key] = val
			}
		}
	}
}
