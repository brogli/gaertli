# Dependencies

Own code is code to test and maintain (G7), and every dependency is code to trust.

- **Reuse before build.** An established library or proven pattern comes first; own code only when
  nothing fits, for a stated reason. Prefer, in order: the standard library or framework core, its
  official extensions, an established third-party library, own code.
- **Established** means maintained (activity in the last year), widely used, and under a licence on
  the licence check's allow-list (OSS-01). The licence rule covers everything that ships or is
  linked into a test binary, not dev tools that are only run.
- **Adapted code** comes only from permissive projects, ships their notice and names its source in a
  comment (OSS-05). Copyleft and source-available code is never read.
- **Pinned.** Every dependency and tool is pinned exactly, so each update is a change of its own
  that passes the checks (QA-MNT-04). Automated updates take only releases past a waiting period, so
  a compromised release is usually withdrawn first; breaking updates wait for review. Wails' Go
  module and JS runtime move together, by hand.
