import  React from 'react'
import { useOvermind } from "../overmind";


const Info = () => {
    const { state } = useOvermind()

    return (
        <div className="box">
        <h4>Welcome to AG</h4>
        </div>
    )
}

export default Info