package web

import (
	"context"
	"errors"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/quickfeed/quickfeed/ci"
	"github.com/quickfeed/quickfeed/database"
	"github.com/quickfeed/quickfeed/qf"
	"github.com/quickfeed/quickfeed/scm"
)

// QuickFeedService holds references to the database and
// other shared data structures.
type QuickFeedService struct {
	logger *zap.SugaredLogger
	db     database.Database
	scmMgr *scm.Manager
	bh     BaseHookOptions
	runner ci.Runner
	qf.UnimplementedQuickFeedServiceServer
}

// NewQuickFeedService returns a QuickFeedService object.
func NewQuickFeedService(logger *zap.Logger, db database.Database, mgr *scm.Manager, bh BaseHookOptions, runner ci.Runner) *QuickFeedService {
	return &QuickFeedService{
		logger: logger.Sugar(),
		db:     db,
		scmMgr: mgr,
		bh:     bh,
		runner: runner,
	}
}

// GetUser will return current user with active course enrollments
// to use in separating teacher and admin roles
func (s *QuickFeedService) GetUser(ctx context.Context, _ *qf.Void) (*qf.User, error) {
	usr, err := s.getCurrentUser(ctx)
	if err != nil {
		s.logger.Errorf("GetUser failed: authentication error: %v", err)
		return nil, ErrInvalidUserInfo
	}
	userInfo, err := s.db.GetUserWithEnrollments(usr.GetID())
	if err != nil {
		s.logger.Errorf("GetUser failed to get user with enrollments: %v ", err)
		return nil, ErrInvalidUserInfo
	}
	return userInfo, nil
}

// GetUsers returns a list of all users.
// Frontend note: This method is called from AdminPage.
func (s *QuickFeedService) GetUsers(_ context.Context, _ *qf.Void) (*qf.Users, error) {
	users, err := s.getUsers()
	if err != nil {
		s.logger.Errorf("GetUsers failed: %v", err)
		return nil, status.Error(codes.NotFound, "failed to get users")
	}
	return users, nil
}

// GetUserByCourse returns the user matching the given course name and GitHub login
// specified in CourseUserRequest.
func (s *QuickFeedService) GetUserByCourse(_ context.Context, in *qf.CourseUserRequest) (*qf.User, error) {
	userInfo, err := s.getUserByCourse(in)
	if err != nil {
		s.logger.Errorf("GetUserByCourse failed: %+v", err)
		return nil, status.Error(codes.FailedPrecondition, "failed to get student information")
	}
	return userInfo, nil
}

// UpdateUser updates the current users's information and returns the updated user.
// This function can also promote a user to admin or demote a user.
func (s *QuickFeedService) UpdateUser(ctx context.Context, in *qf.User) (*qf.Void, error) {
	usr, err := s.getCurrentUser(ctx)
	if err != nil {
		s.logger.Errorf("UpdateUser failed: authentication error: %v", err)
		return nil, ErrInvalidUserInfo
	}
	if _, err = s.updateUser(usr, in); err != nil {
		s.logger.Errorf("UpdateUser failed to update user %d: %v", in.GetID(), err)
		err = status.Error(codes.InvalidArgument, "failed to update user")
	}
	return &qf.Void{}, err
}

// CreateCourse creates a new course.
func (s *QuickFeedService) CreateCourse(ctx context.Context, in *qf.Course) (*qf.Course, error) {
	usr, err := s.getCurrentUser(ctx)
	if err != nil {
		s.logger.Errorf("CreateCourse failed: user authentication error: %v", err)
		return nil, ErrInvalidUserInfo
	}
	scmClient, err := s.getSCM(ctx, in.OrganizationPath)
	if err != nil {
		s.logger.Errorf("CreateCourse failed: could not create scm client for the course %s: %v", in.Name, err)
		return nil, ErrMissingInstallation
	}
	// make sure that the current user is set as course creator
	in.CourseCreatorID = usr.GetID()
	course, err := s.createCourse(ctx, scmClient, in)
	if err != nil {
		s.logger.Errorf("CreateCourse failed: %v", err)
		// errors informing about requested organization state will have code 9: FailedPrecondition
		// error message will be displayed to the user
		if contextCanceled(ctx) {
			return nil, status.Error(codes.FailedPrecondition, ErrContextCanceled)
		}
		if err == ErrAlreadyExists || err == ErrFreePlan {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		if ok, parsedErr := parseSCMError(err); ok {
			return nil, parsedErr
		}
		return nil, status.Error(codes.InvalidArgument, "failed to create course")
	}
	return course, nil
}

// UpdateCourse changes the course information details.
func (s *QuickFeedService) UpdateCourse(ctx context.Context, in *qf.Course) (*qf.Void, error) {
	scmClient, err := s.getSCM(ctx, in.OrganizationPath)
	if err != nil {
		s.logger.Errorf("CreateCourse failed: could not create scm client for the course %s: %v", in.Name, err)
		return nil, ErrMissingInstallation
	}
	if err = s.updateCourse(ctx, scmClient, in); err != nil {
		s.logger.Errorf("UpdateCourse failed: %v", err)
		if contextCanceled(ctx) {
			return nil, status.Error(codes.FailedPrecondition, ErrContextCanceled)
		}
		if ok, parsedErr := parseSCMError(err); ok {
			return nil, parsedErr
		}
		return nil, status.Error(codes.InvalidArgument, "failed to update course")
	}
	return &qf.Void{}, nil
}

// GetCourse returns course information for the given course.
func (s *QuickFeedService) GetCourse(_ context.Context, in *qf.CourseRequest) (*qf.Course, error) {
	courseID := in.GetCourseID()
	course, err := s.getCourse(courseID)
	if err != nil {
		s.logger.Errorf("GetCourse failed: %v", err)
		return nil, status.Error(codes.NotFound, "course not found")
	}
	return course, nil
}

// GetCourses returns a list of all courses.
func (s *QuickFeedService) GetCourses(_ context.Context, _ *qf.Void) (*qf.Courses, error) {
	courses, err := s.getCourses()
	if err != nil {
		s.logger.Errorf("GetCourses failed: %v", err)
		return nil, status.Error(codes.NotFound, "no courses found")
	}
	return courses, nil
}

// UpdateCourseVisibility allows to edit what courses are visible in the sidebar.
func (s *QuickFeedService) UpdateCourseVisibility(_ context.Context, in *qf.Enrollment) (*qf.Void, error) {
	err := s.changeCourseVisibility(in)
	if err != nil {
		s.logger.Errorf("ChangeCourseVisibility failed: %v", err)
		err = status.Error(codes.InvalidArgument, "failed to update course visibility")
	}
	return &qf.Void{}, err
}

// CreateEnrollment enrolls a new student for the course specified in the request.
func (s *QuickFeedService) CreateEnrollment(_ context.Context, in *qf.Enrollment) (*qf.Void, error) {
	err := s.createEnrollment(in)
	if err != nil {
		s.logger.Errorf("CreateEnrollment failed: %v", err)
		err = status.Error(codes.InvalidArgument, "failed to create enrollment")
	}
	return &qf.Void{}, err
}

// UpdateEnrollments changes status of all pending enrollments for the specified course to approved.
// If the request contains a single enrollment, it will be updated to the specified status.
func (s *QuickFeedService) UpdateEnrollments(ctx context.Context, in *qf.Enrollments) (*qf.Void, error) {
	usr, err := s.getCurrentUser(ctx)
	if err != nil {
		s.logger.Errorf("UpdateEnrollments failed: scm authentication error: %v", err)
		return nil, ErrInvalidUserInfo
	}
	scmClient, err := s.getSCMForCourse(ctx, in.Enrollments[0].GetCourseID())
	if err != nil {
		s.logger.Errorf("UpdateEnrollments failed: could not create scm client: %v", err)
		return nil, ErrMissingInstallation
	}
	for _, enrollment := range in.GetEnrollments() {
		if s.isCourseCreator(enrollment.CourseID, enrollment.UserID) {
			s.logger.Errorf("UpdateEnrollments failed: user %s attempted to demote course creator", usr.GetName())
			return nil, status.Error(codes.PermissionDenied, "course creator cannot be demoted")
		}
		if err = s.updateEnrollment(ctx, scmClient, usr.GetLogin(), enrollment); err != nil {
			s.logger.Errorf("UpdateEnrollments failed: %v", err)
			if contextCanceled(ctx) {
				return nil, status.Error(codes.FailedPrecondition, ErrContextCanceled)
			}
			if ok, parsedErr := parseSCMError(err); ok {
				return nil, parsedErr
			}
			return nil, status.Error(codes.InvalidArgument, "failed to update enrollment")
		}
	}
	return &qf.Void{}, err
}

// GetCoursesByUser returns all courses the given user is enrolled into with the given status.
func (s *QuickFeedService) GetCoursesByUser(_ context.Context, in *qf.EnrollmentStatusRequest) (*qf.Courses, error) {
	courses, err := s.getCoursesByUser(in)
	if err != nil {
		s.logger.Errorf("GetCoursesWithEnrollment failed: %v", err)
		return nil, status.Error(codes.NotFound, "no courses with enrollment found")
	}
	return courses, nil
}

// GetEnrollmentsByUser returns all enrollments for the given user and enrollment status with preloaded courses and groups.
func (s *QuickFeedService) GetEnrollmentsByUser(_ context.Context, in *qf.EnrollmentStatusRequest) (*qf.Enrollments, error) {
	// get all enrollments from the db (no scm)
	enrols, err := s.getEnrollmentsByUser(in)
	if err != nil {
		s.logger.Errorf("Get enrollments for user %d failed: %v", in.GetUserID(), err)
	}
	return enrols, nil
}

// GetEnrollmentsByCourse returns all enrollments for the course specified in the request.
func (s *QuickFeedService) GetEnrollmentsByCourse(_ context.Context, in *qf.EnrollmentRequest) (*qf.Enrollments, error) {
	enrolls, err := s.getEnrollmentsByCourse(in)
	if err != nil {
		s.logger.Errorf("GetEnrollmentsByCourse failed: %v", err)
		return nil, status.Error(codes.InvalidArgument, "failed to get enrollments for given course")
	}
	return enrolls, nil
}

// GetGroup returns information about a group.
func (s *QuickFeedService) GetGroup(_ context.Context, in *qf.GetGroupRequest) (*qf.Group, error) {
	group, err := s.getGroup(in)
	if err != nil {
		s.logger.Errorf("GetGroup failed: %v", err)
		return nil, status.Error(codes.NotFound, "failed to get group")
	}
	return group, nil
}

// GetGroupsByCourse returns a list of groups created for the course id in the record request.
func (s *QuickFeedService) GetGroupsByCourse(_ context.Context, in *qf.CourseRequest) (*qf.Groups, error) {
	groups, err := s.getGroups(in)
	if err != nil {
		s.logger.Errorf("GetGroups failed: %v", err)
		return nil, status.Error(codes.NotFound, "failed to get groups")
	}
	return groups, nil
}

// GetGroupByUserAndCourse returns the group of the given student for a given course.
func (s *QuickFeedService) GetGroupByUserAndCourse(_ context.Context, in *qf.GroupRequest) (*qf.Group, error) {
	group, err := s.getGroupByUserAndCourse(in)
	if err != nil {
		if err != errUserNotInGroup {
			s.logger.Errorf("GetGroupByUserAndCourse failed: %v", err)
		}
		return nil, status.Error(codes.NotFound, "failed to get group for given user and course")
	}
	return group, nil
}

// CreateGroup creates a new group in the database.
// Access policy: Any User enrolled in course and specified as member of the group or a course teacher.
func (s *QuickFeedService) CreateGroup(_ context.Context, in *qf.Group) (*qf.Group, error) {
	group, err := s.createGroup(in)
	if err != nil {
		s.logger.Errorf("CreateGroup failed: %v", err)
		if _, ok := status.FromError(err); ok {
			// err was already a status error; return it to client.
			return nil, err
		}
		// err was not a status error; return a generic error to client.
		return nil, status.Error(codes.InvalidArgument, "failed to create group")
	}
	return group, nil
}

// UpdateGroup updates group information, and returns the updated group.
func (s *QuickFeedService) UpdateGroup(ctx context.Context, in *qf.Group) (*qf.Group, error) {
	scmClient, err := s.getSCMForCourse(ctx, in.GetCourseID())
	if err != nil {
		s.logger.Errorf("UpdateGroup failed: could not create scm client for group %s and course %d: %v", in.GetName(), in.GetCourseID(), err)
		return nil, ErrMissingInstallation
	}
	err = s.updateGroup(ctx, scmClient, in)
	if err != nil {
		s.logger.Errorf("UpdateGroup failed: %v", err)
		if contextCanceled(ctx) {
			return nil, status.Error(codes.FailedPrecondition, ErrContextCanceled)
		}
		if ok, parsedErr := parseSCMError(err); ok {
			return nil, parsedErr
		}
		if _, ok := status.FromError(err); ok {
			// err was already a status error; return it to client.
			return nil, err
		}
		// err was not a status error; return a generic error to client.
		return nil, status.Error(codes.InvalidArgument, "failed to update group")
	}
	group, err := s.db.GetGroup(in.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get group")
	}
	return group, nil
}

// DeleteGroup removes group record from the database.
func (s *QuickFeedService) DeleteGroup(ctx context.Context, in *qf.GroupRequest) (*qf.Void, error) {
	scmClient, err := s.getSCMForCourse(ctx, in.GetCourseID())
	if err != nil {
		s.logger.Errorf("DeleteGroup failed: could not create scm client for group %d and course %d: %v", in.GetGroupID(), in.GetCourseID(), err)
		return nil, ErrMissingInstallation
	}
	if err = s.deleteGroup(ctx, scmClient, in); err != nil {
		s.logger.Errorf("DeleteGroup failed: %v", err)
		if contextCanceled(ctx) {
			return nil, status.Error(codes.FailedPrecondition, ErrContextCanceled)
		}
		if ok, parsedErr := parseSCMError(errors.Unwrap(err)); ok {
			return nil, parsedErr
		}
		return nil, status.Error(codes.InvalidArgument, "failed to delete group")
	}
	return &qf.Void{}, nil
}

// GetSubmissions returns the submissions matching the query encoded in the action request.
func (s *QuickFeedService) GetSubmissions(ctx context.Context, in *qf.SubmissionRequest) (*qf.Submissions, error) {
	usr, err := s.getCurrentUser(ctx)
	if err != nil {
		s.logger.Errorf("GetSubmissions failed: authentication error: %v", err)
		return nil, ErrInvalidUserInfo
	}
	s.logger.Debugf("GetSubmissions: %v", in)
	submissions, err := s.getSubmissions(in)
	if err != nil {
		s.logger.Errorf("GetSubmissions failed: %v", err)
		return nil, status.Error(codes.NotFound, "no submissions found")
	}
	// If the user is not a teacher, remove score and reviews from submissions that are not released.
	if !s.isTeacher(usr.ID, in.CourseID) {
		submissions.Clean()
	}
	return submissions, nil
}

// GetSubmissionsByCourse returns all the latest submissions
// for every individual or group course assignment for all course students/groups.
func (s *QuickFeedService) GetSubmissionsByCourse(_ context.Context, in *qf.SubmissionsForCourseRequest) (*qf.CourseSubmissions, error) {
	s.logger.Debugf("GetSubmissionsByCourse: %v", in)
	courseLinks, err := s.getAllCourseSubmissions(in)
	if err != nil {
		s.logger.Errorf("GetSubmissionsByCourse failed: %v", err)
		return nil, status.Error(codes.NotFound, "no submissions found")
	}
	return courseLinks, nil
}

// UpdateSubmission is called to approve the given submission or to undo approval.
func (s *QuickFeedService) UpdateSubmission(_ context.Context, in *qf.UpdateSubmissionRequest) (*qf.Void, error) {
	if !s.isValidSubmission(in.SubmissionID) {
		s.logger.Errorf("UpdateSubmission failed: submission author has no access to the course")
		return nil, status.Error(codes.PermissionDenied, "submission author has no course access")
	}
	err := s.updateSubmission(in.GetCourseID(), in.GetSubmissionID(), in.GetStatus(), in.GetReleased(), in.GetScore())
	if err != nil {
		s.logger.Errorf("UpdateSubmission failed: %v", err)
		err = status.Error(codes.InvalidArgument, "failed to approve submission")
	}
	return &qf.Void{}, err
}

// RebuildSubmissions re-runs the tests for the given assignment.
// A single submission is executed again if the request specifies a submission ID
// or all submissions if the request specifies a course ID.
func (s *QuickFeedService) RebuildSubmissions(_ context.Context, in *qf.RebuildRequest) (*qf.Void, error) {
	// RebuildType can be either SubmissionID or CourseID, but not both.
	switch in.GetRebuildType().(type) {
	case *qf.RebuildRequest_SubmissionID:
		if !s.isValidSubmission(in.GetSubmissionID()) {
			s.logger.Errorf("RebuildSubmission failed: submitter has no access to the course")
			return nil, status.Error(codes.PermissionDenied, "submitter has no course access")
		}
		if _, err := s.rebuildSubmission(in); err != nil {
			s.logger.Errorf("RebuildSubmission failed: %v", err)
			return nil, status.Error(codes.InvalidArgument, "failed to rebuild submission "+err.Error())
		}
	case *qf.RebuildRequest_CourseID:
		if err := s.rebuildSubmissions(in); err != nil {
			s.logger.Errorf("RebuildSubmissions failed: %v", err)
			return nil, status.Error(codes.InvalidArgument, "failed to rebuild submissions "+err.Error())
		}
	}
	return &qf.Void{}, nil
}

// CreateBenchmark adds a new grading benchmark for an assignment.
func (s *QuickFeedService) CreateBenchmark(_ context.Context, in *qf.GradingBenchmark) (*qf.GradingBenchmark, error) {
	bm, err := s.createBenchmark(in)
	if err != nil {
		s.logger.Errorf("CreateBenchmark failed for %+v: %v", in, err)
		return nil, status.Error(codes.InvalidArgument, "failed to add benchmark")
	}
	return bm, nil
}

// UpdateBenchmark edits a grading benchmark for an assignment.
func (s *QuickFeedService) UpdateBenchmark(_ context.Context, in *qf.GradingBenchmark) (*qf.Void, error) {
	err := s.updateBenchmark(in)
	if err != nil {
		s.logger.Errorf("UpdateBenchmark failed for %+v: %v", in, err)
		err = status.Error(codes.InvalidArgument, "failed to update benchmark")
	}
	return &qf.Void{}, err
}

// DeleteBenchmark removes a grading benchmark.
func (s *QuickFeedService) DeleteBenchmark(_ context.Context, in *qf.GradingBenchmark) (*qf.Void, error) {
	err := s.deleteBenchmark(in)
	if err != nil {
		s.logger.Errorf("DeleteBenchmark failed for %+v: %v", in, err)
		err = status.Error(codes.InvalidArgument, "failed to delete benchmark")
	}
	return &qf.Void{}, err
}

// CreateCriterion adds a new grading criterion for an assignment.
func (s *QuickFeedService) CreateCriterion(_ context.Context, in *qf.GradingCriterion) (*qf.GradingCriterion, error) {
	c, err := s.createCriterion(in)
	if err != nil {
		s.logger.Errorf("CreateCriterion failed for %+v: %v", in, err)
		return nil, status.Error(codes.InvalidArgument, "failed to add criterion")
	}
	return c, nil
}

// UpdateCriterion edits a grading criterion for an assignment.
func (s *QuickFeedService) UpdateCriterion(_ context.Context, in *qf.GradingCriterion) (*qf.Void, error) {
	err := s.updateCriterion(in)
	if err != nil {
		s.logger.Errorf("UpdateCriterion failed for %+v: %v", in, err)
		err = status.Error(codes.InvalidArgument, "failed to update criterion")
	}
	return &qf.Void{}, err
}

// DeleteCriterion removes a grading criterion for an assignment.
func (s *QuickFeedService) DeleteCriterion(_ context.Context, in *qf.GradingCriterion) (*qf.Void, error) {
	err := s.deleteCriterion(in)
	if err != nil {
		s.logger.Errorf("DeleteCriterion failed for %+v: %v", in, err)
		err = status.Error(codes.InvalidArgument, "failed to delete criterion")
	}
	return &qf.Void{}, err
}

// CreateReview adds a new submission review.
func (s *QuickFeedService) CreateReview(_ context.Context, in *qf.ReviewRequest) (*qf.Review, error) {
	review, err := s.createReview(in.Review)
	if err != nil {
		s.logger.Errorf("CreateReview failed for review %+v: %v", in, err)
		return nil, status.Error(codes.InvalidArgument, "failed to create review")
	}
	return review, nil
}

// UpdateReview updates a submission review.
func (s *QuickFeedService) UpdateReview(_ context.Context, in *qf.ReviewRequest) (*qf.Review, error) {
	review, err := s.updateReview(in.Review)
	if err != nil {
		s.logger.Errorf("UpdateReview failed for review %+v: %v", in, err)
		err = status.Error(codes.InvalidArgument, "failed to update review")
	}
	return review, err
}

// UpdateSubmissions approves and/or releases all manual reviews for student submission for the given assignment
// with the given score.
func (s *QuickFeedService) UpdateSubmissions(_ context.Context, in *qf.UpdateSubmissionsRequest) (*qf.Void, error) {
	err := s.updateSubmissions(in)
	if err != nil {
		s.logger.Errorf("UpdateSubmissions failed for request %+v", in)
		err = status.Error(codes.InvalidArgument, "failed to update submissions")
	}
	return &qf.Void{}, err
}

// GetReviewers returns names of all active reviewers for a student submission.
func (s *QuickFeedService) GetReviewers(_ context.Context, in *qf.SubmissionReviewersRequest) (*qf.Reviewers, error) {
	reviewers, err := s.getReviewers(in.SubmissionID)
	if err != nil {
		s.logger.Errorf("GetReviewers failed: error fetching from database: %v", err)
		return nil, status.Error(codes.InvalidArgument, "failed to get reviewers")
	}
	return &qf.Reviewers{Reviewers: reviewers}, err
}

// GetAssignments returns a list of all assignments for the given course.
func (s *QuickFeedService) GetAssignments(_ context.Context, in *qf.CourseRequest) (*qf.Assignments, error) {
	courseID := in.GetCourseID()
	assignments, err := s.getAssignments(courseID)
	if err != nil {
		s.logger.Errorf("GetAssignments failed: %v", err)
		return nil, status.Error(codes.NotFound, "no assignments found for course")
	}
	return assignments, nil
}

// UpdateAssignments updates the assignments record in the database
// by fetching assignment information from the course's test repository.
func (s *QuickFeedService) UpdateAssignments(_ context.Context, in *qf.CourseRequest) (*qf.Void, error) {
	err := s.updateAssignments(in.GetCourseID())
	if err != nil {
		s.logger.Errorf("UpdateAssignments failed: %v", err)
		return nil, status.Error(codes.NotFound, "course not found")
	}
	return &qf.Void{}, nil
}

// GetOrganization fetches a github organization by name.
func (s *QuickFeedService) GetOrganization(ctx context.Context, in *qf.OrgRequest) (*qf.Organization, error) {
	usr, err := s.getCurrentUser(ctx)
	if err != nil {
		s.logger.Errorf("GetOrganization failed: scm authentication error: %v", err)
		return nil, err
	}
	scmClient, err := s.getSCM(ctx, in.GetOrgName())
	if err != nil {
		s.logger.Errorf("GetOrganization failed: could not create scm client for organization %s: %v", in.GetOrgName(), err)
		return nil, ErrMissingInstallation
	}
	org, err := s.getOrganization(ctx, scmClient, in.GetOrgName(), usr.GetLogin())
	if err != nil {
		s.logger.Errorf("GetOrganization failed: %v", err)
		if contextCanceled(ctx) {
			return nil, status.Error(codes.FailedPrecondition, ErrContextCanceled)
		}
		if err == scm.ErrNotMember {
			return nil, status.Error(codes.NotFound, "organization membership not confirmed, please enable third-party access")
		}
		if err == ErrFreePlan || err == ErrAlreadyExists || err == scm.ErrNotOwner {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		if ok, parsedErr := parseSCMError(err); ok {
			return nil, parsedErr
		}
		return nil, status.Error(codes.NotFound, "organization not found. Please make sure that 3rd-party access is enabled for your organization")
	}
	return org, nil
}

// GetRepositories returns URL strings for repositories of given type for the given course.
func (s *QuickFeedService) GetRepositories(ctx context.Context, in *qf.URLRequest) (*qf.Repositories, error) {
	usr, err := s.getCurrentUser(ctx)
	if err != nil {
		s.logger.Errorf("GetRepositories failed: authentication error: %v", err)
		return nil, ErrInvalidUserInfo
	}

	course, err := s.getCourse(in.GetCourseID())
	if err != nil {
		s.logger.Errorf("GetRepositories failed: course %d not found: %v", in.GetCourseID(), err)
		return nil, status.Error(codes.NotFound, "course not found")
	}

	enrol, _ := s.db.GetEnrollmentByCourseAndUser(course.GetID(), usr.GetID())

	urls := make(map[string]string)
	for _, repoType := range in.GetRepoTypes() {
		var id uint64
		switch repoType {
		case qf.Repository_USER:
			id = usr.GetID()
		case qf.Repository_GROUP:
			id = enrol.GetGroupID() // will be 0 if not enrolled in a group
		}
		repo, _ := s.getRepo(course, id, repoType)
		// for repo == nil: will result in an empty URL string, which will be ignored by the frontend
		urls[repoType.String()] = repo.GetHTMLURL()
	}
	return &qf.Repositories{URLs: urls}, nil
}

// IsEmptyRepo ensures that group repository is empty and can be deleted.
func (s *QuickFeedService) IsEmptyRepo(ctx context.Context, in *qf.RepositoryRequest) (*qf.Void, error) {
	scmClient, err := s.getSCMForCourse(ctx, in.GetCourseID())
	if err != nil {
		s.logger.Errorf("IsEmptyRepo failed: could not create scm client for course %d: %v", in.GetCourseID(), err)
		return nil, ErrMissingInstallation
	}

	if err := s.isEmptyRepo(ctx, scmClient, in); err != nil {
		s.logger.Errorf("IsEmptyRepo failed: %v", err)
		if contextCanceled(ctx) {
			return nil, status.Error(codes.FailedPrecondition, ErrContextCanceled)
		}
		if ok, parsedErr := parseSCMError(err); ok {
			return nil, parsedErr
		}
		return nil, status.Error(codes.FailedPrecondition, "group repository does not exist or not empty")
	}
	return &qf.Void{}, nil
}
