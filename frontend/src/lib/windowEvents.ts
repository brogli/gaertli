import { onScopeDispose } from 'vue'
import { Events, Window } from '@wailsio/runtime'
import type { ChangeScope } from '../../bindings/github.com/brogli/nebegit/internal/core'

// Wails hands every window's event to every window, naming the window it
// is for as the sender. The name is asked on the first event: a server-mode
// page has none until its event connection is up.
let ownName: Promise<string> | undefined

// onWindowEvent registers handler for the events of name meant for this
// window and returns its unlisten function.
export function onWindowEvent<E extends Events.WailsEventName>(
  name: E,
  handler: Events.WailsEventCallback<E>,
): () => void {
  return Events.On(name, (ev) => {
    ownName ??= Window.Name()
    void ownName.then((own) => {
      if (ev.sender === own) handler(ev)
    })
  })
}

// useWindowEvent is onWindowEvent for the calling component's lifetime.
export function useWindowEvent<E extends Events.WailsEventName>(
  name: E,
  handler: Events.WailsEventCallback<E>,
): void {
  onScopeDispose(onWindowEvent(name, handler))
}

// onRepoChanged registers handler for this window's repo:changed events of
// any of scopes and returns its unlisten function.
export function onRepoChanged(scopes: readonly ChangeScope[], handler: () => void): () => void {
  return onWindowEvent('repo:changed', (ev) => {
    if (scopes.includes(ev.data.scope)) handler()
  })
}

// useRepoChanged is onRepoChanged for the calling component's lifetime.
export function useRepoChanged(scopes: readonly ChangeScope[], handler: () => void): void {
  onScopeDispose(onRepoChanged(scopes, handler))
}
