import * as matchers from "@testing-library/jest-dom/matchers";
import crypto from "crypto";
import { useMemo } from "react";
import { vi, beforeAll, afterAll, afterEach, expect } from "vitest";
import type { Region } from "api/typesGenerated";
import type { ProxyLatencyReport } from "contexts/useProxyLatency";
import { server } from "testHelpers/server";

declare module "vitest" {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any -- we don't have control over this type
  interface Assertion<T = any>
    extends jest.Matchers<void, T>,
      matchers.TestingLibraryMatchers<T, void> {}
}

expect.extend(matchers);

// useProxyLatency does some http requests to determine latency.
// This would fail unit testing, or at least make it very slow with
// actual network requests. So just globally mock this hook.
vi.mock("contexts/useProxyLatency", () => ({
  useProxyLatency: (proxies?: Region[]) => {
    // Must use `useMemo` here to avoid infinite loop.
    // Mocking the hook with a hook.
    const proxyLatencies = useMemo(() => {
      if (!proxies) {
        return {} as Record<string, ProxyLatencyReport>;
      }
      return proxies.reduce(
        (acc, proxy) => {
          acc[proxy.id] = {
            accurate: true,
            // Return a constant latency of 8ms.
            // If you make this random it could break stories.
            latencyMS: 8,
            at: new Date(),
          };
          return acc;
        },
        {} as Record<string, ProxyLatencyReport>,
      );
    }, [proxies]);

    return { proxyLatencies, refetch: vi.fn() };
  },
}));

global.scrollTo = () => {};
window.HTMLElement.prototype.scrollIntoView = vi.fn();
window.open = vi.fn();

// Polyfill the getRandomValues that is used on utils/random.ts
Object.defineProperty(global.self, "crypto", {
  value: {
    getRandomValues: function (buffer: Buffer) {
      return crypto.randomFillSync(buffer);
    },
  },
});

// Establish API mocking before all tests through MSW.
beforeAll(() =>
  server.listen({
    onUnhandledRequest: "warn",
  }),
);

// Reset any request handlers that we may add during the tests,
// so they don't affect other tests.
afterEach(() => {
  server.resetHandlers();
  vi.clearAllMocks();
});

// Clean up after the tests are finished.
afterAll(() => server.close());

// This is needed because we are compiling under `--isolatedModules`
export {};
