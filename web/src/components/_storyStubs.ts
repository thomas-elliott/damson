/**
 * Story-scoped stand-ins for browser globals. The Storybook dev UI loads every
 * story module into one page and keeps it, so a patch made at module level
 * would outlive its file and answer another file's requests. Call these from a
 * `beforeEach` and return the result: Storybook runs the teardown before the
 * next story renders.
 */

/** Serves `window.fetch` from `fake`, which gets the real fetch to fall back on. */
export function stubFetch(fake: (realFetch: typeof window.fetch) => typeof window.fetch) {
  const realFetch = window.fetch;
  window.fetch = fake(realFetch);
  return () => {
    window.fetch = realFetch;
  };
}

/** Keeps XHR uploads in flight: requests are opened but never sent. */
export function holdXhrUploads() {
  const realSend = XMLHttpRequest.prototype.send;
  XMLHttpRequest.prototype.send = function send() {};
  return () => {
    XMLHttpRequest.prototype.send = realSend;
  };
}
