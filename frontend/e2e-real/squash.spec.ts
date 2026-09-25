import { test, expect } from './fixtures'
import { contextMenuItems, logRow, notice, pickFromContextMenu, rev, shortRev } from './helpers'

// Squash and fixup from the log's context menu, on the real repo: the
// component tests mock the core, so only here does a menu choice run the
// rebase and the watcher bring the rewritten log back.

test('FR-LOG-14 a merge commit between the selected commits blocks squash and fixup, and the menu says so', async ({
  app,
  repos,
  git,
}) => {
  await logRow(app, 'add logo and func a').click()
  await logRow(app, 'docs and func b').click({ modifiers: ['Control'] })
  await logRow(app, 'main: change conflict.txt').click({ modifiers: ['Control'] })
  const merge = shortRev(git, repos.work, 'HEAD')

  const items = await contextMenuItems(app, logRow(app, 'docs and func b'))

  expect(items).toEqual([
    'Cherry-pick 3 commits',
    'Cherry-pick 3 commits…',
    'Revert 3 commits',
    'Revert 3 commits…',
    `Squash and fixup not possible: the merge commit ${merge} is in the way`,
    'Copy 3 hashes',
  ])
})

test.describe('a repo with no remote', () => {
  test.use({ open: ['scratch'] })

  test('FR-LOG-13, FR-LOG-14 three ctrl-picked commits squash into one with the edited message', async ({
    openApp,
    repos,
    git,
  }) => {
    const app = await openApp(repos.scratch)
    const head = rev(git, repos.scratch, 'HEAD')
    await logRow(app, 'commit B').click()
    await logRow(app, 'commit C').click({ modifiers: ['Control'] })
    await logRow(app, 'commit A').click({ modifiers: ['Control'] })
    await expect(app.getByText('3 commits selected')).toBeVisible()

    await pickFromContextMenu(app, logRow(app, 'commit C'), 'Squash 3 commits…')
    const dialog = app.getByRole('dialog')
    const message = dialog.getByLabel('Message of the new commit')
    await expect(message).toHaveValue('commit A\n\ncommit B\n\ncommit C')
    await message.fill('A to C, folded')
    await dialog.getByRole('button', { name: 'Squash' }).click()

    await expect(logRow(app, 'A to C, folded')).toBeVisible()
    await expect(logRow(app, 'commit B')).toBeHidden()
    expect(git(repos.scratch, 'log', '--format=%B').trim()).toBe('A to C, folded')
    await expect(notice(app)).toContainText(
      `Squashed 3 commits on main. Previous commit: ${head.slice(0, 7)}`,
    )
  })
})
