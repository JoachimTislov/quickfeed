package web_test

import (
	"context"
	"log"
	"net"
	"testing"
	"time"

	"github.com/quickfeed/quickfeed/database"
	"github.com/quickfeed/quickfeed/internal/qtest"
	"github.com/quickfeed/quickfeed/qf"
	"github.com/quickfeed/quickfeed/qlog"
	"github.com/quickfeed/quickfeed/web"
	"github.com/quickfeed/quickfeed/web/auth"
	"github.com/quickfeed/quickfeed/web/auth/tokens"
	"github.com/quickfeed/quickfeed/web/interceptor"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

const (
	grpcAddr = "127.0.0.1:9090"
	token    = "some-secret-string"
	// same as quickfeed root user
	//botUserID = 1
	userName = "meling"
)

var user *qf.User

func TestGrpcAuth(t *testing.T) {
	db, cleanup, _, qfService := testQuickFeedService(t)
	defer cleanup()

	fillDatabase(t, db)
	if user.Login != userName {
		t.Errorf("Expected %v, got %v\n", userName, user.Login)
	}

	tm, err := tokens.NewTokenManager(db, "test")
	if err != nil {
		t.Fatalf("failed to create token manager: %v", err)
	}

	// start gRPC server in background
	go startGrpcAuthServer(t, qfService, tm)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatalf("failed to connect to grpc server: %v", err)
	}
	defer conn.Close()

	client := qf.NewQuickFeedServiceClient(conn)

	// create request context with the helpbot's secret token
	reqCtx := metadata.NewOutgoingContext(ctx,
		metadata.New(map[string]string{auth.Cookie: token}),
	)

	request := &qf.CourseUserRequest{
		CourseCode: "DAT320",
		CourseYear: 2021,
		UserLogin:  userName,
	}
	userInfo, err := client.GetUserByCourse(reqCtx, request)
	check(t, err)
	if userInfo.ID != user.ID {
		t.Errorf("expected user id %d, got %d", user.ID, userInfo.ID)
	}
	if userInfo.Login != user.Login {
		t.Errorf("expected user login %s, got %s", user.Login, userInfo.Login)
	}
}

func fillDatabase(t *testing.T, db database.Database) {
	// TODO(vera): fix bot compatibility.
	// Add secret token for the helpbot application (to allow it to invoke gRPC methods)
	//auth.Add(token, botUserID)

	// Check that token was stored and maps to correct user
	// checkCookie := auth.Get(token)
	// if checkCookie != botUserID {
	// 	t.Errorf("Expected %v, got %v\n", botUserID, checkCookie)
	// }
	admin := qtest.CreateFakeUser(t, db, 1)
	course := &qf.Course{
		Code: "DAT320",
		Name: "Operating Systems and Systems Programming",
		Year: 2021,
	}
	qtest.CreateCourse(t, db, admin, course)

	user = qtest.CreateUser(t, db, 11, &qf.User{Login: userName})
	qtest.EnrollStudent(t, db, user, course)
}

func startGrpcAuthServer(t *testing.T, qfService *web.QuickFeedService, tm *tokens.TokenManager) {
	lis, err := net.Listen("tcp", grpcAddr)
	check(t, err)

	// TODO(vera): needs update.
	opt := grpc.ChainUnaryInterceptor(
		interceptor.UnaryUserVerifier(qlog.Logger(t), tm),
	)
	grpcServer := grpc.NewServer(opt)

	qf.RegisterQuickFeedServiceServer(grpcServer, qfService)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to start grpc server: %v\n", err)
	}
}

func check(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
