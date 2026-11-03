// tieScroll keeps items' scrollers together: when one scrolls, follow moves each
// of the others after it. A write fires a scroll event of its own, which reports
// the position written and is ignored, so the scrollers never bounce off each
// other; a write that moves nothing fires none and is not waited for. Returns
// untie.
export function tieScroll<T>(
  items: T[],
  scrollerOf: (item: T) => HTMLElement,
  follow: (from: T, to: T) => void,
): () => void {
  const echoes = new Map<T, ScrollPosition>()

  const listeners = items.map((from) => {
    const scroller = scrollerOf(from)
    const onScroll = (): void => {
      const echo = echoes.get(from)
      if (echo && samePosition(echo, positionOf(scroller))) {
        echoes.delete(from)
        return
      }
      for (const to of items) {
        if (to === from) continue
        const before = positionOf(scrollerOf(to))
        follow(from, to)
        const after = positionOf(scrollerOf(to))
        if (!samePosition(before, after)) echoes.set(to, after)
      }
    }
    scroller.addEventListener('scroll', onScroll, { passive: true })
    return () => scroller.removeEventListener('scroll', onScroll)
  })
  return () => listeners.forEach((untie) => untie())
}

// syncScroll ties two scrollers to the same position on both axes. The browser
// clamps a write to the receiver's own range, so a longer side scrolls on past
// a shorter side's end.
export function syncScroll(a: HTMLElement, b: HTMLElement): () => void {
  return tieScroll(
    [a, b],
    (scroller) => scroller,
    (from, to) => {
      to.scrollLeft = from.scrollLeft
      to.scrollTop = from.scrollTop
    },
  )
}

interface ScrollPosition {
  left: number
  top: number
}

function positionOf(el: HTMLElement): ScrollPosition {
  return { left: el.scrollLeft, top: el.scrollTop }
}

function samePosition(a: ScrollPosition, b: ScrollPosition): boolean {
  return a.left === b.left && a.top === b.top
}
