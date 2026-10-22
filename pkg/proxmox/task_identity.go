package proxmox

// SameExecutionIdentity compares observations of one opaque execution. Status
// and completion time may advance between reads; its guest/parent, process,
// start time and owner cannot. Callers bind omitted node labels to the request
// scope before comparing. No UPID parsing or missing-field guessing is done.
func (task Task) SameExecutionIdentity(other Task) bool {
	task.Status, other.Status = "", ""
	task.EndTime, other.EndTime = 0, 0
	return task == other
}
