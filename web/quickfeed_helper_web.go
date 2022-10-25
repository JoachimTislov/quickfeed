package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bufbuild/connect-go"
	"github.com/quickfeed/quickfeed/ci"
	"github.com/quickfeed/quickfeed/database"
	"github.com/quickfeed/quickfeed/internal/qtest"
	"github.com/quickfeed/quickfeed/qf/qfconnect"
	"github.com/quickfeed/quickfeed/scm"
)

// testQuickFeedService is a clone of the same function in quickfeed_helper_test.go.
// It is replicated here to avoid import cycle.
func testQuickFeedService(t *testing.T) (database.Database, func(), scm.SCM, *QuickFeedService) {
	t.Helper()
	db, cleanup := qtest.TestDB(t)
	sc, mgr := scm.MockSCMManager(t)
	logger := qtest.Logger(t).Desugar()
	return db, cleanup, sc, NewQuickFeedService(logger, db, mgr, BaseHookOptions{}, &ci.Local{})
}

// MockClient returns a QuickFeed client for invoking RPCs.
func MockClient(t *testing.T, db database.Database, opts connect.Option) qfconnect.QuickFeedServiceClient {
	t.Helper()
	_, mgr := scm.MockSCMManager(t)
	logger := qtest.Logger(t)
	qfService := NewQuickFeedService(logger.Desugar(), db, mgr, BaseHookOptions{}, &ci.Local{})

	if opts == nil {
		opts = connect.WithInterceptors()
	}
	router := http.NewServeMux()
	router.Handle(qfconnect.NewQuickFeedServiceHandler(qfService, opts))
	server := httptest.NewUnstartedServer(router)
	server.EnableHTTP2 = true
	server.StartTLS()
	t.Cleanup(server.Close)

	return qfconnect.NewQuickFeedServiceClient(server.Client(), server.URL)
}
