package scm

const (
	getByID                                         = "GET /organizations/{id}"
	getOrgsByOrg                                    = "GET /orgs/{org}"
	patchOrgsByOrg                                  = "PATCH /orgs/{org}"
	getOrgsReposByOrg                               = "GET /orgs/{org}/repos"
	postOrgsReposByOrg                              = "POST /orgs/{org}/repos"
	getOrgsMembershipsByOrgByUsername               = "GET /orgs/{org}/memberships/{username}"
	getReposOwnerByOwnerByRepo                      = "GET /repos/{owner}/{repo}"
	getReposContentsByOwnerByRepoByPath             = "GET /repos/{owner}/{repo}/contents/{path...}"
	getReposCollaboratorsByOwnerByRepo              = "GET /repos/{owner}/{repo}/collaborators"
	putReposCollaboratorsByOwnerByRepoByUsername    = "PUT /repos/{owner}/{repo}/collaborators/{username}"
	deleteReposCollaboratorsByOwnerByRepoByUsername = "DELETE /repos/{owner}/{repo}/collaborators/{username}"
	postIssueByOwnerByRepo                          = "POST /repos/{owner}/{repo}/issues"
	patchIssueByOwnerByRepoByIssueNumber            = "PATCH /repos/{owner}/{repo}/issues/{issue_number}"
	getIssueByOwnerByRepoByIssueNumber              = "GET /repos/{owner}/{repo}/issues/{issue_number}"
	getIssuesByOwnerByRepo                          = "GET /repos/{owner}/{repo}/issues"
	postIssueCommentByOwnerByRepoByIssueNumber      = "POST /repos/{owner}/{repo}/issues/{issue_number}/comments"
	patchIssueCommentByOwnerByRepoByCommentID       = "PATCH /repos/{owner}/{repo}/issues/comments/{comment_id}"
	postPullReviewersByOwnerByRepoByPullNumber      = "POST /repos/{owner}/{repo}/pulls/{pull_number}/requested_reviewers"
)
