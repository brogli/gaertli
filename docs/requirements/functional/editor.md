# External editor

The app is editor-neutral.

| ID | Prio | Requirement |
|---|---|---|
| FR-ED-01 | MUST | Open a worktree folder in the external editor. |
| FR-ED-02 | MUST | Open a file in the external editor from the change list, the diff, or the conflict list. |
| FR-ED-11 | MUST | The editor is set per environment, because the command runs where the repo lives: the local system and each installed WSL distro always have their own editor setting, named after the environment, and a repo uses its own environment's. |
| FR-ED-03 | MUST | The user configures which editor is used; every "open in editor" action uses this one setting. Settings label it as the setting behind "Open in …". |
| FR-ED-04 | MUST | The user can enter any custom command, with separate forms for "open folder" and "open file" (file form may include line). |
| FR-ED-05 | MUST | Default is the editor. The UI says "Open in editor", or shows the configured editor's name, never a fixed "Open in the editor". A saved change shows at once in every window. |
| FR-ED-06 | MUST | If the editor cannot be started, a notice says so and names the command that failed, and nothing more. |
| FR-ED-07 | SHOULD | Presets for common editors (e.g. the editor, another editor, an IDE, a third editor, a terminal editor in a terminal) so the user only picks from a list. |
| FR-ED-09 | SHOULD | Open the file at the line shown in the diff, if the editor command supports it. |
