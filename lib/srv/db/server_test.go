/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package db

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/proto"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/multiplexer"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"
	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
)

func TestDatabaseAccess(t *testing.T) {
	utils.InitLoggerForTests(testing.Verbose())

	ctx := context.Background()
	testCtx := setupTestContext(ctx, t)
	defer testCtx.Close()

	// Start multiplexer.
	go testCtx.mux.Serve()
	// Start fake Postgres server.
	go testCtx.postgresServer.Serve()
	// Start database proxy server.
	go testCtx.proxyServer.Serve(testCtx.mux.DB())
	// Start database service server.
	go func() {
		for conn := range testCtx.proxyConn {
			testCtx.server.HandleConnection(conn)
		}
	}()

	tests := []struct {
		desc         string
		user         string
		role         string
		allowDbNames []string
		allowDbUsers []string
		dbName       string
		dbUser       string
		err          string
	}{
		{
			desc:         "has access to all database names and users",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{services.Wildcard},
			allowDbUsers: []string{services.Wildcard},
			dbName:       "postgres",
			dbUser:       "postgres",
		},
		{
			desc:         "has access to nothing",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{},
			allowDbUsers: []string{},
			dbName:       "postgres",
			dbUser:       "postgres",
			err:          "access to database denied",
		},
		{
			desc:         "no access to databases",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{},
			allowDbUsers: []string{services.Wildcard},
			dbName:       "postgres",
			dbUser:       "postgres",
			err:          "access to database denied",
		},
		{
			desc:         "no access to users",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{services.Wildcard},
			allowDbUsers: []string{},
			dbName:       "postgres",
			dbUser:       "postgres",
			err:          "access to database denied",
		},
		{
			desc:         "access allowed to specific user/database",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{"metrics"},
			allowDbUsers: []string{"alice"},
			dbName:       "metrics",
			dbUser:       "alice",
		},
		{
			desc:         "access denied to specific user/database",
			user:         "alice",
			role:         "admin",
			allowDbNames: []string{"metrics"},
			allowDbUsers: []string{"alice"},
			dbName:       "postgres",
			dbUser:       "postgres",
			err:          "access to database denied",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			// Create user/role with the requested permissions.
			_, role, err := auth.CreateUserAndRole(testCtx.tlsServer.Auth(), test.user, []string{test.role})
			require.NoError(t, err)

			role.SetDatabaseNames(services.Allow, test.allowDbNames)
			role.SetDatabaseUsers(services.Allow, test.allowDbUsers)
			err = testCtx.tlsServer.Auth().UpsertRole(ctx, role)
			require.NoError(t, err)

			// Try to connect to the database as this user.
			pgConn, err := connectToPostgres(ctx, testCtx, connectConfig{
				service: "test",
				user:    test.user,
				dbName:  test.dbName,
				dbUser:  test.dbUser,
			})
			if test.err != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.err)
			} else {
				require.NoError(t, err)

				// Execute a query.
				result, err := pgConn.Exec(ctx, "select 1").ReadAll()
				require.NoError(t, err)
				require.Equal(t, []*pgconn.Result{fakeQueryResponse}, result)

				// Disconnect.
				err = pgConn.Close(ctx)
				require.NoError(t, err)
			}
		})
	}
}

type testContext struct {
	clusterName    string
	tlsServer      *auth.TestTLSServer
	authServer     *auth.Server
	authClient     *auth.Client
	postgresServer *PostgresServer
	proxyServer    *ProxyServer
	mux            *multiplexer.Mux
	proxyConn      chan (net.Conn)
	server         *Server
}

// Close closes all resources associated with the test context.
func (c *testContext) Close() error {
	if c.mux != nil {
		c.mux.Close()
	}
	if c.postgresServer != nil {
		c.postgresServer.Close()
	}
	if c.server != nil {
		c.server.Close()
	}
	return nil
}

func setupTestContext(ctx context.Context, t *testing.T) *testContext {
	clusterName := "root.example.com"
	hostID := uuid.New()

	// Create multiplexer.
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	mux, err := multiplexer.New(multiplexer.Config{ID: "test", Listener: listener})
	require.NoError(t, err)

	// Create and start test auth server.
	authServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{ClusterName: clusterName, Dir: t.TempDir()})
	require.NoError(t, err)
	tlsServer, err := authServer.NewTestTLSServer()
	require.NoError(t, err)

	// Use sync recording to not involve the uploader.
	clusterConfig, err := authServer.AuthServer.GetClusterConfig()
	require.NoError(t, err)
	clusterConfig.SetSessionRecording(services.RecordAtNodeSync)
	err = authServer.AuthServer.SetClusterConfig(clusterConfig)
	require.NoError(t, err)

	// Auth client/authorizer for database service.
	dbAuthClient, err := tlsServer.NewClient(auth.TestServerID(teleport.RoleDatabase, hostID))
	require.NoError(t, err)
	dbAuthorizer, err := auth.NewAuthorizer(dbAuthClient, dbAuthClient, dbAuthClient)
	require.NoError(t, err)

	// Auth client/authorizer for database proxy.
	proxyAuthClient, err := tlsServer.NewClient(auth.TestBuiltin(teleport.RoleProxy))
	require.NoError(t, err)
	proxyAuthorizer, err := auth.NewAuthorizer(proxyAuthClient, proxyAuthClient, proxyAuthClient)
	require.NoError(t, err)

	// TLS config for database proxy and database service.
	serverIdentity, err := auth.NewServerIdentity(authServer.AuthServer, hostID, teleport.RoleDatabase)
	require.NoError(t, err)
	tlsConfig, err := serverIdentity.TLSConfig(nil)
	require.NoError(t, err)

	// Fake Postgres server that speaks part of its wire protocol.
	postgresServer := setupPostgresServer(ctx, t, dbAuthClient)

	// Create a database server for the test database service.
	dbServer := makeDatabaseServer(hostID, fmt.Sprintf("localhost:%v", postgresServer.Port()))
	_, err = dbAuthClient.UpsertDatabaseServer(ctx, dbServer)
	require.NoError(t, err)

	// Establish fake reversetunnel b/w database proxy and database service.
	connCh := make(chan net.Conn)
	tunnel := &reversetunnel.FakeServer{
		Sites: []reversetunnel.RemoteSite{
			&reversetunnel.FakeRemoteSite{
				Name:        clusterName,
				ConnCh:      connCh,
				AccessPoint: proxyAuthClient,
			},
		},
	}

	// Create database proxy server.
	proxyServer, err := NewProxyServer(ctx, ProxyServerConfig{
		AuthClient:  proxyAuthClient,
		AccessPoint: proxyAuthClient,
		Authorizer:  proxyAuthorizer,
		Tunnel:      tunnel,
		TLSConfig:   tlsConfig,
	})
	require.NoError(t, err)

	// Create database service server.
	server, err := New(ctx, Config{
		Clock:         clockwork.NewFakeClock(),
		DataDir:       t.TempDir(),
		AuthClient:    dbAuthClient,
		AccessPoint:   dbAuthClient,
		StreamEmitter: dbAuthClient,
		Authorizer:    dbAuthorizer,
		Server:        dbServer,
		TLSConfig:     tlsConfig,
		CipherSuites:  utils.DefaultCipherSuites(),
		GetRotation:   func(teleport.Role) (*services.Rotation, error) { return &services.Rotation{}, nil },
		OnHeartbeat:   func(error) {},
	})
	require.NoError(t, err)

	return &testContext{
		clusterName:    clusterName,
		mux:            mux,
		proxyServer:    proxyServer,
		proxyConn:      connCh,
		postgresServer: postgresServer,
		server:         server,
		tlsServer:      tlsServer,
		authServer:     tlsServer.Auth(),
		authClient:     dbAuthClient,
	}
}

// setupPostgresServer creates a fake Postgres server to be used in tests.
func setupPostgresServer(ctx context.Context, t *testing.T, authClient *auth.Client) *PostgresServer {
	key, err := client.NewKey()
	require.NoError(t, err)
	csr, err := tlsca.GenerateCertificateRequestPEM(pkix.Name{CommonName: "localhost"}, key.Priv)
	require.NoError(t, err)
	resp, err := authClient.GenerateDatabaseCert(ctx,
		&proto.DatabaseCertRequest{
			CSR:        csr,
			ServerName: "localhost",
			TTL:        proto.Duration(time.Hour),
		})
	require.NoError(t, err)
	cert, err := tls.X509KeyPair(resp.Cert, key.Priv)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	for _, ca := range resp.CACerts {
		require.True(t, pool.AppendCertsFromPEM(ca))
	}
	postgresServer, err := NewPostgresServer(PostgresServerConfig{
		TLSConfig: &tls.Config{
			ClientCAs:    pool,
			Certificates: []tls.Certificate{cert},
		},
	})
	require.NoError(t, err)
	return postgresServer
}

type connectConfig struct {
	service string
	user    string
	dbName  string
	dbUser  string
}

func connectToPostgres(ctx context.Context, testCtx *testContext, config connectConfig) (*pgconn.PgConn, error) {
	// Client will be connecting directly to the multiplexer address.
	pgconnConfig, err := pgconn.ParseConfig(fmt.Sprintf("postgres://%v@%v/?database=%v", config.dbUser, testCtx.mux.DB().Addr(), config.dbName))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key, err := client.NewKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Generate client certificate for the Teleport user.
	cert, err := testCtx.authServer.GenerateDatabaseTestCert(key.Pub, testCtx.clusterName, config.service, config.user, time.Hour)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCert, err := tls.X509KeyPair(cert, key.Priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ca, err := testCtx.authClient.GetCertAuthority(services.CertAuthID{Type: services.HostCA, DomainName: testCtx.clusterName}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pool, err := services.CertPool(ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pgconnConfig.TLSConfig = &tls.Config{
		RootCAs:            pool,
		Certificates:       []tls.Certificate{tlsCert},
		InsecureSkipVerify: true,
	}
	pgConn, err := pgconn.ConnectConfig(ctx, pgconnConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pgConn, nil
}

func makeDatabaseServer(name, uri string) *services.ServerV2 {
	return &services.ServerV2{
		Kind:    services.KindDatabaseServer,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
		},
		Spec: services.ServerSpecV2{
			Version:  teleport.Version,
			Hostname: teleport.APIDomain,
			Databases: []*services.Database{
				{
					Name:     "test",
					Protocol: defaults.ProtocolPostgres,
					URI:      uri,
				},
			},
		},
	}
}
