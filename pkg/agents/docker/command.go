package dockeragent

// Command represents a control instruction issued by Pulse to the Docker agent.
type Command struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
}

// ReportResponse captures the server response for a docker report submission.
type ReportResponse struct {
	Success  bool      `json:"success"`
	Commands []Command `json:"commands,omitempty"`
}

// CommandAck is sent by the agent to confirm the result of a control command.
type CommandAck struct {
	HostID  string `json:"hostId"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

const (
	// CommandTypeStop instructs the agent to stop reporting and shut down.
	CommandTypeStop = "stop"
	// CommandTypeUpdateContainer instructs the agent to update a specific container to its latest image.
	CommandTypeUpdateContainer = "update_container"

	// CommandStatusAcknowledged indicates a command was received and is in progress.
	CommandStatusAcknowledged = "acknowledged"
	// CommandStatusCompleted indicates the command completed successfully.
	CommandStatusCompleted = "completed"
	// CommandStatusFailed indicates the command failed.
	CommandStatusFailed = "failed"
)
