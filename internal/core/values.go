package core

import "strings"

// SHA is a commit's full object name as git prints it: 40 or 64 lowercase
// hex digits. The zero SHA means no commit (an unborn HEAD, a base side
// without one).
type SHA string

func (s SHA) Validate() error {
	if n := len(s); n != 40 && n != 64 || strings.ContainsFunc(string(s), notLowerHex) {
		return Errorf(Internal, "%q is not a commit hash", s)
	}
	return nil
}

func notLowerHex(r rune) bool { return ('0' > r || r > '9') && ('a' > r || r > 'f') }

// RefName is a ref's full name in one of the three namespaces the app
// handles: refs/heads/<branch>, refs/remotes/<remote>/<branch>,
// refs/tags/<tag>. Full, because a bare name that is both a tag and a
// branch is taken as the tag by some git commands and refused by others.
type RefName string

const (
	branchNamespace       = "refs/heads/"
	remoteBranchNamespace = "refs/remotes/"
	tagNamespace          = "refs/tags/"
)

func (r RefName) Validate() error {
	short := r.Short()
	noRemoteBranch := r.IsRemoteBranch() && !strings.Contains(short, "/")
	if short == "" || strings.HasSuffix(short, "/") || noRemoteBranch {
		return Errorf(Internal, "%q is not a full ref name", r)
	}
	return nil
}

// Short is the name without its namespace, "" outside the three: what the
// UI shows and what git is given where a full name would be refused or
// would change a message.
func (r RefName) Short() string {
	for _, ns := range []string{branchNamespace, remoteBranchNamespace, tagNamespace} {
		if short, ok := strings.CutPrefix(string(r), ns); ok {
			return short
		}
	}
	return ""
}

func (r RefName) IsBranch() bool       { return strings.HasPrefix(string(r), branchNamespace) }
func (r RefName) IsRemoteBranch() bool { return strings.HasPrefix(string(r), remoteBranchNamespace) }
func (r RefName) IsTag() bool          { return strings.HasPrefix(string(r), tagNamespace) }
