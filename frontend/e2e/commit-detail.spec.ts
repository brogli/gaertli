import { test, expect, type Locator } from '@playwright/test'
import { openMockRepo } from './helpers'

// happy-dom computes no layout or scrolling, so the commit detail view's
// two parts, each scrolling on its own, need a real browser.

const top = async (locator: Locator) => (await locator.boundingBox())!.y

for (const width of [1600, 1200, 420]) {
  test(`FR-LOG-15 at ${width}px a commit's message and its files scroll each on its own`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 800 })
    await openMockRepo(page, '&commitFiles=60')
    await page.locator('.row').first().dblclick()
    const view = page.locator('.commit-details')
    const message = view.locator('.commit-message')
    const files = view.locator('.file-scroll')
    await expect(files.locator('.files li')).toHaveCount(60)

    const subject = view.locator('.subject')
    const subjectTop = await top(subject)
    await files.evaluate((el) => (el.scrollTop = el.scrollHeight))
    expect(await top(subject)).toBe(subjectTop)
    await expect(files.locator('.files li').last()).toBeInViewport()

    const filesHeader = view.locator('.changed-files h3')
    const headerTop = await top(filesHeader)
    await message.evaluate((el) => (el.scrollTop = el.scrollHeight))
    expect(await top(filesHeader)).toBe(headerTop)
    await expect(message.locator('.fields')).toBeInViewport()
  })
}
