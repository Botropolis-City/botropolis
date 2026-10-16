package city

// actionText says what each action does, in a plain line a reader whose
// first language is not English can follow (item 88).
var actionText = map[ActionKind]string{
	ActionAttach:       "Opens this session's terminal.",
	ActionResume:       "Starts this resting session again and opens its terminal.",
	ActionDemolish:     "Deletes this session. Its conversation is gone for good.",
	ActionStop:         "Stops this session. Its conversation is kept, and you can resume it.",
	ActionPack:         "Takes this session off the map and keeps it in the depot. Unpack brings it back.",
	ActionUnpack:       "Brings this session back to its old lot.",
	ActionNew:          "Starts a new session in this project's directory.",
	ActionReveal:       "Opens this project's directory in your file manager.",
	ActionCopyPath:     "Copies this project's path.",
	ActionHide:         "Takes this project off the map. Unhide it from the sidebar.",
	ActionStar:         "Marks this project, so it is easy to find.",
	ActionUnstar:       "Removes the mark from this project.",
	ActionDetails:      "Shows every number this card has.",
	ActionDetailsHide:  "Folds the numbers away again.",
	ActionMoveToGroup:  "Lets you choose which group this session belongs to.",
	ActionMove:         "Moves this session to the group you chose.",
	ActionGenerate:     "Asks Claude for a short summary of this session. This uses tokens.",
	ActionRead:         "Opens the conversation here, to read without leaving the city.",
	ActionCopy:         "Copies this line.",
	ActionReviewGroups: "Says these groups are right, so the city stops asking.",
	ActionFocusLine:    "Shows only this session's group, and frames its projects.",
	ActionCopyStandup:  "Copies one line saying where each of this group's projects stands, for a stand-up.",
	ActionBreakdown:    "Shows where the tokens and money went.",
	ActionOpenPR:       "Opens this session's pull request in your browser.",
	ActionShowRoad:     "Lights the roads from this project to the projects it works with.",
	ActionFork:         "Starts a fresh session from a copy of this conversation. This one stays as it is.",
	ActionNewSession:   "Starts a session in the directory you chose, in the group you chose.",
}

// Describe is what the action does, in one line.
func (k ActionKind) Describe() string { return actionText[k] }
