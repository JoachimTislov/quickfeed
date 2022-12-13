package web_test

import (
	"context"
	"testing"

	"github.com/bufbuild/connect-go"
	"github.com/google/go-cmp/cmp"
	"github.com/quickfeed/quickfeed/internal/qtest"
	"github.com/quickfeed/quickfeed/qf"
	"github.com/quickfeed/quickfeed/web/auth"
	"github.com/quickfeed/quickfeed/web/interceptor"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestGetUsers(t *testing.T) {
	db, cleanup := qtest.TestDB(t)
	defer cleanup()
	client := MockClient(t, db, nil)
	ctx := context.Background()

	unexpectedUsers, err := client.GetUsers(ctx, &connect.Request[qf.Void]{Msg: &qf.Void{}})
	if err == nil && unexpectedUsers != nil && len(unexpectedUsers.Msg.GetUsers()) > 0 {
		t.Fatalf("found unexpected users %+v", unexpectedUsers)
	}

	admin := qtest.CreateFakeUser(t, db, 1)
	user2 := qtest.CreateFakeUser(t, db, 2)

	ctx = auth.WithUserContext(ctx, admin)
	foundUsers, err := client.GetUsers(ctx, &connect.Request[qf.Void]{Msg: &qf.Void{}})
	if err != nil {
		t.Fatal(err)
	}

	wantUsers := make([]*qf.User, 0)
	wantUsers = append(wantUsers, admin, user2)
	gotUsers := foundUsers.Msg.GetUsers()
	if diff := cmp.Diff(wantUsers, gotUsers, protocmp.Transform()); diff != "" {
		t.Errorf("GetUsers() mismatch (-wantUsers +gotUsers):\n%s", diff)
	}
}

var allUsers = []struct {
	remoteID uint64
	secret   string
}{
	{1, "123"},
	{2, "123"},
	{3, "456"},
	{4, "789"},
	{5, "012"},
	{6, "345"},
	{7, "678"},
	{8, "901"},
	{9, "234"},
}

func TestGetEnrollmentsByCourse(t *testing.T) {
	db, cleanup := qtest.TestDB(t)
	defer cleanup()
	client := MockClient(t, db, nil)
	ctx := context.Background()

	var users []*qf.User
	for _, u := range allUsers {
		user := qtest.CreateFakeUser(t, db, u.remoteID)
		users = append(users, user)
	}
	admin := users[0]
	for _, course := range qtest.MockCourses {
		err := db.CreateCourse(admin.ID, course)
		if err != nil {
			t.Fatal(err)
		}
	}

	ctx = auth.WithUserContext(ctx, admin)

	// users to enroll in course DAT520 Distributed Systems
	// (excluding admin because admin is enrolled on creation)
	wantUsers := users[0 : len(allUsers)-3]
	for i, user := range wantUsers {
		if i == 0 {
			// skip enrolling admin as student
			continue
		}
		qtest.EnrollStudent(t, db, user, qtest.MockCourses[0])
	}

	// users to enroll in course DAT320 Operating Systems
	// (excluding admin because admin is enrolled on creation)
	osUsers := users[3:7]
	for _, user := range osUsers {
		qtest.EnrollStudent(t, db, user, qtest.MockCourses[1])
	}

	request := &connect.Request[qf.EnrollmentRequest]{
		Msg: &qf.EnrollmentRequest{
			FetchMode: &qf.EnrollmentRequest_CourseID{
				CourseID: qtest.MockCourses[0].ID,
			},
		},
	}
	gotEnrollments, err := client.GetEnrollments(ctx, request)
	if err != nil {
		t.Error(err)
	}
	var gotUsers []*qf.User
	for _, e := range gotEnrollments.Msg.Enrollments {
		gotUsers = append(gotUsers, e.User)
	}
	if diff := cmp.Diff(wantUsers, gotUsers, protocmp.Transform()); diff != "" {
		t.Errorf("GetEnrollmentsByCourse() mismatch (-wantUsers +gotUsers):\n%s", diff)
	}
}

func TestUpdateUser(t *testing.T) {
	db, cleanup := qtest.TestDB(t)
	defer cleanup()
	logger := qtest.Logger(t)

	tm, err := auth.NewTokenManager(db)
	if err != nil {
		t.Fatal(err)
	}
	client := MockClient(t, db, connect.WithInterceptors(
		interceptor.NewUserInterceptor(logger, tm),
	))
	ctx := context.Background()

	firstAdminUser := qtest.CreateFakeUser(t, db, 1)
	nonAdminUser := qtest.CreateFakeUser(t, db, 11)

	firstAdminCookie, err := tm.NewAuthCookie(firstAdminUser.ID)
	if err != nil {
		t.Fatal(err)
	}

	// we want to update nonAdminUser to become admin
	nonAdminUser.IsAdmin = true
	err = db.UpdateUser(nonAdminUser)
	if err != nil {
		t.Fatal(err)
	}

	// we expect the nonAdminUser to now be admin
	admin, err := db.GetUser(nonAdminUser.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !admin.IsAdmin {
		t.Error("expected nonAdminUser to have become admin")
	}

	nameChangeRequest := connect.NewRequest(&qf.User{
		ID:        nonAdminUser.ID,
		IsAdmin:   nonAdminUser.IsAdmin,
		Name:      "Scrooge McDuck",
		StudentID: "99",
		Email:     "test@test.com",
		AvatarURL: "www.hello.com",
	})

	nameChangeRequest.Header().Set(auth.Cookie, firstAdminCookie.String())
	_, err = client.UpdateUser(ctx, nameChangeRequest)
	if err != nil {
		t.Error(err)
	}
	gotUser, err := db.GetUser(nonAdminUser.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantUser := &qf.User{
		ID:           gotUser.ID,
		Name:         "Scrooge McDuck",
		IsAdmin:      true,
		StudentID:    "99",
		Email:        "test@test.com",
		AvatarURL:    "www.hello.com",
		RefreshToken: nonAdminUser.RefreshToken,
		ScmRemoteID:  nonAdminUser.ScmRemoteID,
	}
	if diff := cmp.Diff(wantUser, gotUser, protocmp.Transform()); diff != "" {
		t.Errorf("UpdateUser() mismatch (-wantUser +gotUser):\n%s", diff)
	}
}

func TestUpdateUserFailures(t *testing.T) {
	db, cleanup := qtest.TestDB(t)
	defer cleanup()
	client, tm, _ := MockClientWithUser(t, db)
	ctx := context.Background()

	admin := qtest.CreateNamedUser(t, db, 1, "admin")
	if !admin.IsAdmin {
		t.Fatalf("expected user %v to be admin", admin)
	}
	user := qtest.CreateNamedUser(t, db, 2, "user")
	if user.IsAdmin {
		t.Fatalf("expected user %v to be non-admin", user)
	}
	userCookie := Cookie(t, tm, user)
	tests := []struct {
		name     string
		cookie   string
		req      *qf.User
		wantUser *qf.User
		wantErr  bool
	}{
		{
			name:   "user demotes admin, must fail",
			cookie: userCookie,
			req: &qf.User{
				ID:        admin.ID,
				IsAdmin:   false,
				Name:      admin.Name,
				Email:     admin.Email,
				StudentID: admin.StudentID,
				AvatarURL: admin.AvatarURL,
			},
			wantUser: nil,
			wantErr:  true,
		},
		{
			name:   "user promotes self to admin, must fail",
			cookie: userCookie,
			req: &qf.User{
				ID:        user.ID,
				Name:      user.Name,
				Email:     user.Email,
				StudentID: user.StudentID,
				AvatarURL: user.AvatarURL,
				IsAdmin:   true,
			},
			wantUser: nil,
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := client.UpdateUser(ctx, qtest.RequestWithCookie(tt.req, tt.cookie))
			if (err != nil) != tt.wantErr {
				t.Errorf("%s: expected error: %v, got = %v", tt.name, tt.wantErr, err)
			}
			if !tt.wantErr {
				if diff := cmp.Diff(tt.wantUser, user, protocmp.Transform()); diff != "" {
					t.Errorf("%s: mismatch users (-want +got):\n%s", tt.name, diff)
				}
			}
		})
	}
}
