import * as React from "react";
import { formatDate } from "../../helper";
import { IStudentLabsForCourse, IStudentLab, ISubmission } from "../../models";
import { ProgressBar } from "../progressbar/ProgressBar";
import { submissionStatusToString } from "../../componentHelper";

interface ISingleCourseOverviewProps {
    courseAndLabs: IStudentLabsForCourse;
    groupAndLabs?: IStudentLabsForCourse;
    onLabClick: (courseId: number, labId: number) => void;
    onGroupLabClick: (courseId: number, labId: number) => void;
}

export class SingleCourseOverview extends React.Component<ISingleCourseOverviewProps> {
    public render() {
        let groupLabs: IStudentLab[] = [];
        if (this.props.groupAndLabs !== undefined) {
            groupLabs = this.props.groupAndLabs.labs;
        }
        let submissionArray = this.buildInfo(this.props.courseAndLabs.labs, groupLabs);

        // Fallback if the length of grouplabs and userlabs is different.
        if (!submissionArray) {
            submissionArray = this.props.courseAndLabs.labs;
        }

        const labs: JSX.Element[] = submissionArray.map((submissionLink, k) => {
            let submissionInfo = <div>No submissions</div>;
            if (submissionLink.submission) {
                submissionInfo = <div className="row">
                    <div className="col-md-6 col-lg-6">
                        <ProgressBar progress={submissionLink.submission.score} />
                    </div>
                    <div className="col-md-2 col-lg-2" >
                        <span className="text-success"> Passed: {submissionLink.submission.passedTests} </span>
                        <span className="text-danger"> Failed: {submissionLink.submission.failedTests} </span>
                    </div>
                    <div className="col-md-2 col-lg-2">
                        <span > {this.setStatusString(submissionLink.submission, submissionLink.assignment.getReviewers() > 0)} </span>
                    </div>
                    <div className="col-md-2 col-lg-2">
                        Deadline:
                        <span style={{ display: "inline-block", verticalAlign: "top", paddingLeft: "10px" }}>
                            {formatDate(submissionLink.assignment.getDeadline())}
                        </span>
                    </div>
                </div>;
            }
            return (
                <li key={k} className="list-group-item clickable"
                    // Testing if the onClick handler should be for studentlab or grouplab.
                    onClick={() => {
                        const courseId = submissionLink.assignment.getCourseid();
                        const assignmentId = submissionLink.assignment.getId();
                        if (!submissionLink.assignment.getIsgrouplab()) {
                            return this.props.onLabClick(courseId, assignmentId);
                        } else {
                            return this.props.onGroupLabClick(courseId, assignmentId);
                        }
                    }}>
                    <strong>{submissionLink.assignment.getName()}</strong>
                    {submissionInfo}
                </li >);
        });
        return (
            <div>
                <h1>{this.props.courseAndLabs.course.getName()}</h1>
                <div>
                    <ul className="list-group">
                        {labs}
                    </ul>
                </div>
            </div >
        );
    }
    private buildInfo(studentLabs: IStudentLab[], groupLabs: IStudentLab[]):
     IStudentLab[] | null {
        const labAndGrouplabs: IStudentLab[] = [];
        if (studentLabs.length !== groupLabs.length) {
            return null;
        }
        for (let labCounter = 0; labCounter < studentLabs.length; labCounter++) {
            if (!studentLabs[labCounter].assignment.getIsgrouplab()) {
                labAndGrouplabs.push(studentLabs[labCounter]);
            } else {
                labAndGrouplabs.push(groupLabs[labCounter]);
            }
        }
        return labAndGrouplabs;
    }

    private setStatusString(submission: ISubmission, manualReview: boolean): string {
        if (manualReview) {
            return submissionStatusToString(submission.status);
        }
        return submission.approved ? "Approved" : "Not approved";
    }
}
