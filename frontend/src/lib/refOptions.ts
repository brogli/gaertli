import type { Refs } from '../../bindings/github.com/brogli/gaertli/internal/core'
import type { Translate } from '@/i18n'

// RefOption is one ref in a Select or MultiSelect: its short name shown,
// its full ref name the value, so a branch and a tag of the same name stay
// apart.
export interface RefOption {
  label: string
  value: string
}

// refGroups lists refs as option groups: local branches, remote branches,
// tags. An empty group is left out.
export function refGroups(
  refs: Refs | null,
  t: Translate,
): { label: string; items: RefOption[] }[] {
  const local = (refs?.branches ?? []).map((b) => ({
    label: b.name,
    value: `refs/heads/${b.name}`,
  }))
  const remote = (refs?.remotes ?? []).flatMap((r) =>
    (r.branches ?? []).map((b) => ({
      label: `${r.name}/${b.name}`,
      value: `refs/remotes/${r.name}/${b.name}`,
    })),
  )
  const tags = (refs?.tags ?? []).map((tag) => ({
    label: tag.name,
    value: `refs/tags/${tag.name}`,
  }))
  return [
    { label: t('refs.local'), items: local },
    { label: t('refs.remote'), items: remote },
    { label: t('refs.tags'), items: tags },
  ].filter((group) => group.items.length)
}

// branchSuggestions lists the repo's branch names (local and remote, short
// names, deduplicated) containing the query, then the query itself, so
// Enter (which picks the first, focused suggestion) always turns the typed
// text into a chip. A name in chosen is never suggested.
export function branchSuggestions(refs: Refs | null, query: string, chosen: string[]): string[] {
  const local = (refs?.branches ?? []).map((b) => b.name)
  const remote = (refs?.remotes ?? []).flatMap((r) => (r.branches ?? []).map((b) => b.name))
  const lower = query.toLowerCase()
  const matches = [...local, ...remote].filter((name) => name.toLowerCase().includes(lower))
  return [...new Set([...matches, query])].filter((name) => !chosen.includes(name))
}
