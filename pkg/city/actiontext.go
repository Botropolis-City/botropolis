package city

// actionText says what each action does, in a plain line a reader whose
// first language is not English can follow (item 88).
var actionText = map[ActionKind]string{
	ActionAttach:        "Opens this session's terminal.",
	ActionResume:        "Starts this resting session again and opens its terminal.",
	ActionDemolish:      "Deletes this session and its worktree, and takes it off the map. Its conversation stays in Claude Code's files.",
	ActionStop:          "Stops this session. Its conversation is kept, and you can resume it.",
	ActionPack:          "Takes this session off the map and keeps it in the depot. Unpack brings it back.",
	ActionUnpack:        "Brings this session back to its old lot.",
	ActionRestore:       "Reopens in the background the sessions that were open when you left.",
	ActionCloseUp:       "Notes what is open so you can pick up where you left off, and pauses only the sessions you choose.",
	ActionNew:           "Starts a new session in this project's directory.",
	ActionReveal:        "Opens this project's directory in your file manager.",
	ActionCopyPath:      "Copies this project's path.",
	ActionHide:          "Takes this project off the map. Unhide it from the sidebar.",
	ActionStar:          "Marks this project, so it is easy to find.",
	ActionUnstar:        "Removes the mark from this project.",
	ActionDistrictType:  "Changes what kind of buildings this project's district grows.",
	ActionDetails:       "Shows every number this card has.",
	ActionDetailsHide:   "Folds the numbers away again.",
	ActionMoveToGroup:   "Moves this session into a group, or into a new one you name.",
	ActionMove:          "Moves this session to the group you chose.",
	ActionJoinSuggested: "Moves this session into the group it seems to belong to.",
	ActionTakeOut:       "Takes this session out of its group, and keeps it out.",
	ActionRenameGroup:   "Gives this session's group a name you choose, which the city keeps.",
	ActionRename:        "Gives the group the name you typed.",
	ActionCloseGroup:    "Packs every session in this group into the depot and takes the group off the board. Unpack any of them to bring it back.",
	ActionGenerate:      "Asks Claude for a short summary of this session. This uses tokens.",
	ActionRead:          "Opens the conversation here, to read without leaving the city.",
	ActionCopy:          "Copies this line.",
	ActionReviewGroups:  "Says these groups are right, so the city stops asking.",
	ActionFocusLine:     "Shows only this session's group, and frames its projects.",
	ActionCopyStandup:   "Copies one line saying where each of this group's projects stands, for a stand-up.",
	ActionMuteWord:      "Stops a word that joined this session to its group from joining sessions again.",
	ActionMute:          "Stops this word from joining sessions into groups.",
	ActionBreakdown:     "Shows where the tokens and money went.",
	ActionOpenPR:        "Opens this session's pull request in your browser.",
	ActionShowRoad:      "Lights the roads from this project to the projects it works with.",
	ActionFork:          "Starts a fresh session from a copy of this conversation. This one stays as it is.",
	ActionAskWhere:      "Asks which directory to start a session in.",
	ActionAskDemolish:   "Asks before this session is demolished.",
	ActionNewSession:    "Starts a session in the directory you chose, in the group you chose.",
}

// teamActionText is what the actions that act on a whole team do, said
// on the card of a session in one (item 343).
var teamActionText = map[ActionKind]string{
	ActionMoveToGroup:   "Moves this session and its team into a group, or into a new one you name.",
	ActionJoinSuggested: "Moves this session and its team into the group it seems to belong to.",
	ActionTakeOut:       "Takes this session and its team out of their group, and keeps them out.",
}

// Describe is what the action does, in one line.
func (k ActionKind) Describe() string { return actionText[k] }

// describeFor is what the action does for the selection: on a session in
// a team, a move or a take-out acts on the team.
func (k ActionKind) describeFor(inTeam bool) string {
	if text, ok := teamActionText[k]; ok && inTeam {
		return text
	}
	return actionText[k]
}
