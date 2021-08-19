import React, { useState } from "react"
import { useEffect } from "react"
import { Redirect } from "react-router-dom"
import { EnrollmentStatus, EnrollmentStatusBadge, getCourseID, isTeacher, sortByField } from "../Helpers"
import { useAppState, useActions, useGrpc } from "../overmind"
import { Enrollment, User } from "../../proto/ag/ag_pb"
import Search from "./Search"
import DynamicTable from "./DynamicTable"


// TODO: Clean up 

export const Members = () => {
    const state = useAppState()
    const actions = useActions()
    const courseID = getCourseID()

    const [func, setFunc] = useState("STATUS")
    const [descending, setDescending] = useState(false)
    const [edit, setEditing] = useState<boolean>(false)
    useEffect(() => {

    }, [func, setFunc])

    const sort = (): Function => {
        switch (func) {
            case "STATUS":
                return Enrollment.prototype.getStatus
            case "ID":
                return Enrollment.prototype.getId
            default:
                return Enrollment.prototype.getStatus
        }

    }

    const approveAll = () => {
        for (const enrollment of pending) {
            actions.updateEnrollment({enrollment: enrollment, status: Enrollment.UserStatus.STUDENT})
        }
    }

    if (!isTeacher(state.enrollmentsByCourseId[courseID])) {
        return <Redirect to="/" />
    }

    const pending = state.courseEnrollments[courseID].filter(enrollment => enrollment.getStatus() === Enrollment.UserStatus.PENDING)
    
    const members = sortByField(state.courseEnrollments[courseID], [], sort(), descending).map((enrollment: Enrollment) => {
        const demoteText = `Warning! ${enrollment.getUser()?.getName()} is a teacher. Are sure you want to demote?`
        const promoteText = `Are you sure you want to promote ${enrollment.getUser()?.getName()} to teacher status?`
        
        const data: (string | JSX.Element)[] = []
        data.push(enrollment.hasUser() ? (enrollment.getUser() as User).getName() : "")
        data.push(enrollment.hasUser() ? (enrollment.getUser() as User).getEmail() : "")
        data.push(enrollment.hasUser() ? (enrollment.getUser() as User).getStudentid() : "")
        data.push(enrollment.getLastactivitydate())
        data.push(enrollment.getTotalapproved().toString())
        if (enrollment.getStatus() === Enrollment.UserStatus.PENDING) {
            data.push(
                <div>
                    <i className="badge badge-primary" style={{cursor: "pointer"}} onClick={() => actions.updateEnrollment({enrollment: enrollment, status: Enrollment.UserStatus.STUDENT})}>
                        Accept
                    </i>
                    <i 
                        className="badge badge-danger"
                        style={{cursor: "pointer"}} 
                        onClick={() => {
                            if (confirm("WARNNG! Rejecting a student is irreversible. Are you sure?"))
                                actions.updateEnrollment({enrollment: enrollment, status: Enrollment.UserStatus.NONE}) 
                            }}
                    >
                        Reject
                    </i>
                </div>)
        }
        else {
            data.push(edit ? (
                <div>
                    <button 
                        className="btn btn-primary" 
                        onClick={() => confirm(isTeacher(enrollment) ? demoteText : promoteText) ? actions.updateEnrollment({enrollment: enrollment, status: isTeacher(enrollment) ? Enrollment.UserStatus.STUDENT : Enrollment.UserStatus.TEACHER}) : null}
                    >
                        {isTeacher(enrollment) ? "Demote" : "Promote"}
                    </button>
                    <button 
                        className="btn btn-danger" 
                        onClick={() => {
                            if (confirm("WARNNG! Rejecting a student is irreversible. Are you sure?"))
                                actions.updateEnrollment({enrollment: enrollment, status: Enrollment.UserStatus.NONE}) 
                            }}
                    >
                        Reject
                    </button>
                </div>) :
                <i className={EnrollmentStatusBadge[enrollment.getStatus()]}>
                    {EnrollmentStatus[enrollment.getStatus()]}
                </i>
            )
        }
        return data
    })

    return (
        <div className='container'>
            
            
            <div className="float-right">
                <div className={edit ? "btn btn-danger" : "btn btn-primary"} onClick={() => setEditing(!edit)}>{edit ? "Cancel" : "Edit"}</div>
                {pending.length > 0 ? <button className="btn btn-success float-right" onClick={() => approveAll()}>Approve All</button>  : null}
            </div>
            <Search />
            <div>
                <DynamicTable header={["Name", "Email", {value: "Student ID", onClick: () => {setFunc("ID"); setDescending(!descending)}}, "Activity", "Approved", {value: "Role", onClick: () => {setFunc("STATUS"); setDescending(!descending)}}]} data={members} />
            </div>
        </div>
        )
    }


export default Members
