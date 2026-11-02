package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/internal/limiter"
	"muara.bi/core/internal/repository/pgsql"
	"muara.bi/core/pkg/customerror"
	"muara.bi/core/pkg/logging"
	"muara.bi/engine/duckdb"
)

const DefaultEnvTag = "default"

// NormalizeEnvTag maps an empty environment tag to the neutral default used for
// cache keys and connection resolution.
func NormalizeEnvTag(envTag string) string {
	if envTag == "" {
		return DefaultEnvTag
	}
	return envTag
}

// PreparedQuery describes one guardrailed analytical execution.
type PreparedQuery struct {
	WorkspaceID string
	ProjectID   string
	EnvTag      string
	RawSQL      string
	Parameters  map[string]any
	MaxRows     int
	RLSRules    []duckdb.RLSRule
}

// SessionPreparer builds tenant-scoped ephemeral DuckDB sessions and executes
// guardrailed queries inside them. It is shared by the synchronous query API
// and the background workers so both enforce identical attach, RLS and limit
// rules.
type SessionPreparer struct {
	engine       duckdb.Engine
	connRepo     pgsql.ConnectionRepositoryProvider
	semanticRepo pgsql.SemanticRepositoryProvider
	connService  ConnectionServiceProvider
	slots        *limiter.Semaphore
	logger       *logging.Logger
}

func NewSessionPreparer(
	engine duckdb.Engine,
	connRepo pgsql.ConnectionRepositoryProvider,
	semanticRepo pgsql.SemanticRepositoryProvider,
	connService ConnectionServiceProvider,
	slots *limiter.Semaphore,
	logger *logging.Logger,
) *SessionPreparer {
	return &SessionPreparer{
		engine:       engine,
		connRepo:     connRepo,
		semanticRepo: semanticRepo,
		connService:  connService,
		slots:        slots,
		logger:       logger,
	}
}

// Execute validates, parameterizes, row-limits and runs the query inside a
// freshly allocated in-memory session that is destroyed on return.
func (p *SessionPreparer) Execute(ctx context.Context, q PreparedQuery) (*duckdb.QueryResult, error) {
	if err := duckdb.ValidateQuery(q.RawSQL); err != nil {
		return nil, customerror.NewBadRequestError("query rejected by guardrails", err)
	}

	binding, err := duckdb.TransformParameters(q.RawSQL, q.Parameters)
	if err != nil {
		return nil, customerror.NewBadRequestError("failed to bind query parameters", err)
	}

	sqlText := binding.ParameterizedSQL
	args := binding.OrderedArgs
	if len(q.RLSRules) > 0 {
		scopedSQL, rlsArgs, err := duckdb.ApplyRLS(sqlText, q.RLSRules, len(args)+1)
		if err != nil {
			return nil, customerror.NewBadRequestError("row-level security could not be applied", err)
		}
		sqlText = scopedSQL
		args = append(args, rlsArgs...)
	}

	maxRows := q.MaxRows
	if maxRows <= 0 {
		maxRows = duckdb.DefaultMaxRows
	}

	// The budget is taken after the cheap validation and SQL rewrite, so a query
	// that could never run does not consume a slot or make a valid one wait.
	release, err := p.slots.Acquire(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, err
		}
		return nil, customerror.NewTooManyRequestsError(
			"the analytical engine is at capacity, try again shortly", err)
	}
	defer release()

	session, err := p.engine.NewSession(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create analytical session: %w", err)
	}
	defer session.Close()

	if err := p.mountSemanticModels(ctx, session, q); err != nil {
		return nil, err
	}
	cappedSources, err := p.attachConnections(ctx, session, q.WorkspaceID, NormalizeEnvTag(q.EnvTag))
	if err != nil {
		return nil, err
	}

	// The row cap is enforced by the engine, which also reports honestly whether
	// it cut the answer short (BRD §7.3).
	return session.Query(ctx, sqlText, duckdb.QueryOptions{
		MaxRows:       maxRows,
		CappedSources: cappedSources,
	}, args...)
}

func (p *SessionPreparer) mountSemanticModels(ctx context.Context, session duckdb.Session, q PreparedQuery) error {
	if q.ProjectID == "" {
		return nil
	}

	models, err := p.semanticRepo.ListSemanticModelsByProject(ctx, q.WorkspaceID, q.ProjectID)
	if err != nil {
		return fmt.Errorf("failed to load semantic models: %w", err)
	}
	if len(models) == 0 {
		return nil
	}

	smList := make([]duckdb.SemanticModel, 0, len(models))
	for _, m := range models {
		smList = append(smList, duckdb.SemanticModel{
			ID:          m.ID,
			Name:        m.Name,
			Description: m.Description,
			RawSQL:      m.RawSQL,
		})
	}

	if err := duckdb.MountSemanticModels(ctx, session, smList); err != nil {
		return fmt.Errorf("failed to mount semantic models: %w", err)
	}
	return nil
}

// attachConnections binds every workspace alias that has a credential profile
// for the active environment tag. Aliases without a matching profile are
// skipped so one unconfigured environment cannot break unrelated queries.
// It returns the mounted views whose source read was capped, which the executed
// query has to declare as truncation.
func (p *SessionPreparer) attachConnections(ctx context.Context, session duckdb.Session, wsID, envTag string) ([]string, error) {
	conns, err := p.connRepo.ListConnections(ctx, wsID)
	if err != nil {
		return nil, fmt.Errorf("failed to list workspace connections: %w", err)
	}

	attempted, failed := 0, 0
	capped := make([]string, 0)
	var lastErr error
	for _, conn := range conns {
		cfg, _, err := p.connService.GetDecryptedConfig(ctx, wsID, conn.ID, envTag)
		if err != nil || cfg == nil {
			continue
		}
		attempted++
		viewCapped, err := attachConnection(ctx, session, conn, cfg)
		if err != nil {
			failed++
			lastErr = err
			p.logger.Warn(fmt.Sprintf("failed to attach alias %s (env=%s): %v", conn.AliasName, envTag, err))
			continue
		}
		capped = append(capped, viewCapped...)
	}

	// When every source configured for this environment failed, the query would
	// only die later on an unrelated "table does not exist" catalog error.
	if attempted > 0 && failed == attempted {
		return nil, customerror.NewBadRequestError(
			fmt.Sprintf("no data source could be attached for environment %q", envTag),
			lastErr,
		)
	}
	return capped, nil
}

func attachConnection(ctx context.Context, session duckdb.Session, conn entity.DataConnection, cfg *entity.ConnectionConfig) ([]string, error) {
	switch conn.DriverType {
	case "postgres":
		dsn, err := postgresDSN(cfg)
		if err != nil {
			return nil, err
		}
		return nil, session.AttachPostgres(ctx, conn.AliasName, dsn)
	case "mysql":
		// mysql_scanner reads "host=… user=… passwd=…", not the Go driver's
		// `user:pass@tcp(host:port)/db`; the engine owns that grammar so both
		// products emit the same string.
		dsn, err := duckdb.MySQLDSN(duckdb.MySQLSource{
			Host:   cfg.Host,
			Port:   cfg.Port,
			User:   cfg.User,
			Passwd: cfg.Password,
			DB:     cfg.Database,
		})
		if err != nil {
			return nil, err
		}
		return nil, session.AttachMySQL(ctx, conn.AliasName, dsn)
	case "sqlite":
		return nil, session.AttachSQLite(ctx, conn.AliasName, cfg.Database)
	case "mongodb":
		uri, err := mongoURI(cfg)
		if err != nil {
			return nil, err
		}
		mounts, err := duckdb.StreamMongoCollections(ctx, session, duckdb.MongoCollectionStream{
			URI:      uri,
			Database: cfg.Database,
			Alias:    conn.AliasName,
		})
		if err != nil {
			return nil, err
		}
		// A database with no collections mounts nothing, which would otherwise look
		// like a successful attach and then fail on an unknown table.
		if len(mounts) == 0 {
			return nil, fmt.Errorf("mongodb database %q has no collections to mount", cfg.Database)
		}
		return duckdb.CappedMongoViews(mounts), nil
	default:
		return nil, fmt.Errorf("no duckdb attach strategy for driver %q", conn.DriverType)
	}
}

// mongoURI prefers the stored connection string and falls back to the host fields.
// The root MongoDB user lives in `admin`, so authentication is pointed there unless
// the URI already says otherwise.
func mongoURI(cfg *entity.ConnectionConfig) (string, error) {
	tlsOn, err := mongoTLSRequested(cfg.SSLMode)
	if err != nil {
		return "", err
	}
	ca, err := mongoRootCert(cfg.RootCert, tlsOn)
	if err != nil {
		return "", err
	}

	base := cfg.URI
	if base == "" {
		if cfg.Host == "" || cfg.Port == 0 {
			return "", nil
		}
		uri := url.URL{
			Scheme: "mongodb",
			Host:   fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			RawQuery: url.Values{
				"authSource":       []string{"admin"},
				"directConnection": []string{"true"},
			}.Encode(),
		}
		if cfg.User != "" {
			uri.User = url.UserPassword(cfg.User, cfg.Password)
		}
		base = uri.String()
	}

	// Profiles saved before any of this existed carry no TLS wish at all, and
	// rewriting their URI string would change what the operator sees.
	if !tlsOn && ca == "" {
		return base, nil
	}
	return withMongoTLS(base, ca)
}

// mongoTLSRequested maps the stored ssl_mode onto the one thing the Go driver can
// express: TLS on or off. `allow` and `prefer` are libpq fallbacks with no driver
// equivalent, and they are left on the plain side rather than guessed — turning a
// working plaintext source into a failing TLS one is not an improvement the operator
// asked for. What is *not* allowed is keeping a CA next to them, which is where the
// fallback stops being harmless: the file would sit in the profile doing nothing.
func mongoTLSRequested(raw string) (bool, error) {
	switch mode := strings.TrimSpace(raw); mode {
	case "", "disable", "allow", "prefer":
		return false, nil
	case "require", "verify-ca", "verify-full":
		return true, nil
	default:
		return false, fmt.Errorf("ssl_mode %q is not a mode the mongodb driver can be given (want disable, allow, prefer, require, verify-ca or verify-full)", raw)
	}
}

// mongoRootCert is the CA file handed to the driver as tlsCAFile. Measured against
// mongo-driver v1.17.10 (2026-11-02): the named file is opened even when tls is
// off, an empty tlsCAFile= is a failure ("open : no such file"), and the legacy
// sslCAFile is ignored completely — so a CA is only accepted together with TLS.
func mongoRootCert(raw string, tlsOn bool) (string, error) {
	path, err := rootCertPath(raw)
	if err != nil || path == "" {
		return path, err
	}
	if !tlsOn {
		return "", fmt.Errorf("root_cert has no effect while ssl_mode asks for no TLS: the driver would load the file and verify nothing (set ssl_mode to require, verify-ca or verify-full)")
	}
	return path, nil
}

// withMongoTLS adds the TLS wishes to one URI without touching anything else in
// it, because a stored URI may already carry authentication options the product
// must not overwrite.
func withMongoTLS(rawURI, caPath string) (string, error) {
	parsed, err := url.Parse(rawURI)
	if err != nil {
		return "", fmt.Errorf("mongodb uri cannot be parsed: %w", err)
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return "", fmt.Errorf("mongodb uri query cannot be parsed: %w", err)
	}

	// The driver ignores sslCAFile silently, so a profile that names the CA twice
	// — once in the URI under a key nobody reads, once in root_cert — must fail
	// here instead of trusting whichever one the driver happens to look at.
	for _, key := range []string{"sslCAFile", "sslCaFile"} {
		if query.Get(key) != "" {
			return "", fmt.Errorf("mongodb uri sets %s, which the Go driver ignores; drop it and put the CA path in root_cert", key)
		}
	}
	if existing := query.Get("tlsCAFile"); existing != "" && existing != caPath {
		return "", fmt.Errorf("mongodb uri already names tlsCAFile %q, which conflicts with root_cert %q", existing, caPath)
	}
	for _, key := range []string{"ssl", "tls"} {
		if v := query.Get(key); v != "" && v != "true" && caPath != "" {
			return "", fmt.Errorf("mongodb uri sets %s=%s while root_cert is configured: TLS would be off, so the CA cannot verify anything", key, v)
		}
	}

	query.Set("tls", "true")
	if caPath != "" {
		query.Set("tlsCAFile", caPath)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// postgresSSLModes is the closed set of sslmode values libpq accepts. Anything
// else only surfaces as a TLS failure while connecting — after the typo has
// already been saved — so it is refused where the profile is written.
var postgresSSLModes = []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"}

// normalizeSSLMode validates a stored ssl_mode. Empty is a legitimate value and
// becomes the engine default; an unrecognised one is rejected rather than
// silently downgraded, because a misspelled "verify-full" means no cert check.
func normalizeSSLMode(raw string) (string, error) {
	mode := strings.TrimSpace(raw)
	if mode == "" {
		return "disable", nil
	}
	for _, allowed := range postgresSSLModes {
		if mode == allowed {
			return mode, nil
		}
	}
	return "", fmt.Errorf("ssl_mode %q is not a libpq sslmode (want one of: %s)",
		raw, strings.Join(postgresSSLModes, ", "))
}

// postgresVerifyModes are the sslmode values where libpq actually reads a CA
// file. "require" negotiates TLS but accepts any certificate, so a CA saved with
// it would look like verification while nothing is checked.
var postgresVerifyModes = []string{"verify-ca", "verify-full"}

// normalizeStoredRootCert applies the rules of the connector that would actually
// read the file, at the point the profile is written. mysql_scanner was measured
// (2026-11-02, DuckDB 1.1.3) to know only host, port, user, passwd, db and socket —
// ssl-ca, ssl-capath and ssl-mode all come back as "Unrecognized configuration
// parameter" and credentials cannot travel as ATTACH options — so a CA path there
// would be encrypted, stored, and quietly ignored.
func normalizeStoredRootCert(driverType, raw, sslMode string) (string, error) {
	switch driverType {
	case "postgres":
		// libpq opens the CA only for verify-ca / verify-full.
		return normalizeRootCert(raw, sslMode)
	case "mongodb":
		// The Go driver loads tlsCAFile whenever TLS is on, and never when it is off.
		tlsOn, err := mongoTLSRequested(sslMode)
		if err != nil {
			return "", err
		}
		return mongoRootCert(raw, tlsOn)
	default:
		if strings.TrimSpace(raw) == "" {
			return "", nil
		}
		return "", fmt.Errorf("root_cert is not read by the %s connector: only postgres (sslrootcert) and mongodb (tlsCAFile) accept a CA file", driverType)
	}
}

// rootCertPath holds the two checks every driver needs, because both of them
// open the certificate from a file: a relative path resolves against whatever
// working directory the process happens to have, so two environments would load
// two different CAs from one profile, and pasted PEM text is never a path.
func rootCertPath(raw string) (string, error) {
	path := strings.TrimSpace(raw)
	if path == "" {
		return "", nil
	}
	if strings.ContainsAny(path, "\n\x00") {
		return "", fmt.Errorf("root_cert must be a path, not certificate text (want the PEM file, e.g. /etc/ssl/certs/internal-ca.pem)")
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("root_cert %q must be an absolute path — a relative one is resolved against the server's working directory", path)
	}
	return path, nil
}

// normalizeRootCert validates a stored CA path against the libpq mode that would
// read it.
func normalizeRootCert(raw, sslMode string) (string, error) {
	path, err := rootCertPath(raw)
	if err != nil || path == "" {
		return path, err
	}
	for _, allowed := range postgresVerifyModes {
		if sslMode == allowed {
			return path, nil
		}
	}
	return "", fmt.Errorf("root_cert has no effect while ssl_mode is %q: the CA is only read by %s",
		sslMode, strings.Join(postgresVerifyModes, " or "))
}

// postgresDSN builds the libpq conninfo used by both schema discovery and the
// DuckDB ATTACH. sslmode is defaulted in this one place: an empty value produced
// `sslmode= sslmode=disable` before, and pgx rejects that with
// "failed to configure TLS (sslmode is invalid)", so every SyncSchema on a
// connection saved without ssl_mode failed.
func postgresDSN(cfg *entity.ConnectionConfig) (string, error) {
	sslMode, err := normalizeSSLMode(cfg.SSLMode)
	if err != nil {
		return "", err
	}
	rootCert, err := normalizeRootCert(cfg.RootCert, sslMode)
	if err != nil {
		return "", err
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		dsnValue(cfg.Host), cfg.Port, dsnValue(cfg.User), dsnValue(cfg.Password),
		dsnValue(cfg.Database), dsnValue(sslMode))
	if rootCert == "" {
		return dsn, nil
	}
	// sslrootcert, not rootcert: the keyword DuckDB's bundled libpq knows was
	// measured on this engine, and "rootcert" fails as `invalid connection
	// option` — the same message a made-up option produces.
	return dsn + " sslrootcert=" + dsnValue(rootCert), nil
}

// dsnValue quotes one libpq keyword=value entry. Credentials are user supplied,
// so a space would split the value and let the rest be read as another
// parameter; quoting also keeps a password out of the parser's way.
func dsnValue(s string) string {
	if !strings.ContainsAny(s, " \t\n'\\") {
		return s
	}
	escaped := strings.ReplaceAll(s, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `'`, `\'`)
	return "'" + escaped + "'"
}
