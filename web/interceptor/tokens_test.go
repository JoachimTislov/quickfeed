package interceptor_test

import (
	"context"
	"testing"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/golang-jwt/jwt"
	"github.com/quickfeed/quickfeed/internal/qtest"
	"github.com/quickfeed/quickfeed/qf"
	"github.com/quickfeed/quickfeed/web"
	"github.com/quickfeed/quickfeed/web/auth"
	"github.com/quickfeed/quickfeed/web/interceptor"
)

func TestRefreshTokens(t *testing.T) {
	db, cleanup := qtest.TestDB(t)
	defer cleanup()
	logger := qtest.Logger(t)

	tm, err := auth.NewTokenManager(db)
	if err != nil {
		t.Fatal(err)
	}
	client := web.MockClient(t, db, connect.WithInterceptors(
		interceptor.NewUserInterceptor(logger, tm),
		interceptor.NewTokenInterceptor(tm),
	))
	ctx := context.Background()

	f := func(t *testing.T, id uint64) string {
		cookie, err := tm.NewAuthCookie(id)
		if err != nil {
			t.Fatal(err)
		}
		return cookie.String()
	}

	admin := qtest.CreateAdminUser(t, db, "fake")
	user := qtest.CreateFakeUser(t, db, 56)
	adminCookie := f(t, admin.ID)
	userCookie := f(t, user.ID)
	adminClaims := &auth.Claims{
		UserID: admin.ID,
		Admin:  true,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(1 * time.Minute).Unix(),
		},
	}
	userClaims := &auth.Claims{
		UserID: user.ID,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(1 * time.Minute).Unix(),
		},
	}
	if updateRequired(t, tm, userClaims) || updateRequired(t, tm, adminClaims) {
		t.Error("No users should be in the token update list at the start")
	}
	if _, err := client.GetUsers(ctx, qtest.RequestWithCookie(&qf.Void{}, adminCookie)); err != nil {
		t.Fatal(err)
	}
	if updateRequired(t, tm, adminClaims) || updateRequired(t, tm, userClaims) {
		t.Error("No users should be in the token update list")
	}
	if _, err := client.UpdateUser(ctx, qtest.RequestWithCookie(user, adminCookie)); err != nil {
		t.Fatal(err)
	}
	if !updateRequired(t, tm, userClaims) {
		t.Error("User must be in the token update list after admin has updated the user's information")
	}
	if _, err := client.GetUser(ctx, qtest.RequestWithCookie(&qf.Void{}, userCookie)); err != nil {
		t.Fatal(err)
	}
	if updateRequired(t, tm, userClaims) {
		t.Error("User should not be in the token update list after the token has been updated")
	}
	course := &qf.Course{
		ID:               1,
		OrganizationID:   1,
		OrganizationName: qtest.MockOrg,
		Provider:         "fake",
	}
	group := &qf.Group{
		ID:       1,
		Name:     "test",
		CourseID: 1,
		Users: []*qf.User{
			user,
		},
	}
	if _, err := client.CreateCourse(ctx, qtest.RequestWithCookie(course, adminCookie)); err != nil {
		t.Fatal(err)
	}
	if !updateRequired(t, tm, adminClaims) {
		t.Error("Admin must be in the token update list after creating a new course")
	}
	qtest.EnrollStudent(t, db, user, course)
	if _, err := client.CreateGroup(ctx, qtest.RequestWithCookie(group, adminCookie)); err != nil {
		t.Fatal(err)
	}
	if updateRequired(t, tm, userClaims) {
		t.Error("User should not be in the token update list after methods that don't affect the user's information")
	}
	if _, err := client.UpdateGroup(ctx, qtest.RequestWithCookie(group, adminCookie)); err != nil {
		t.Fatal(err)
	}
	if !updateRequired(t, tm, userClaims) {
		t.Error("User must be in the token update group after changes to the group")
	}
	if _, err := client.GetUser(ctx, qtest.RequestWithCookie(&qf.Void{}, userCookie)); err != nil {
		t.Fatal(err)
	}
	if updateRequired(t, tm, userClaims) {
		t.Error("User should be removed from the token update list after the user's token has been updated")
	}
	if _, err := client.DeleteGroup(ctx, qtest.RequestWithCookie(&qf.GroupRequest{
		GroupID:  group.ID,
		CourseID: course.ID,
	}, adminCookie)); err != nil {
		t.Fatal(err)
	}
	if !updateRequired(t, tm, userClaims) {
		t.Error("User must be in the token update list after the group has been deleted")
	}
	if updateRequired(t, tm, adminClaims) {
		t.Error("Admin should not be in the token update list")
	}
}

func updateRequired(t *testing.T, tm *auth.TokenManager, claims *auth.Claims) bool {
	t.Helper()
	updated, err := tm.UpdateCookie(claims)
	if err != nil {
		t.Error(err)
	}
	return updated != nil
}
