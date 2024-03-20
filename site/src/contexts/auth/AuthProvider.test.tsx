import { renderHook } from "@testing-library/react";
import type { FC, PropsWithChildren } from "react";
import { QueryClientProvider } from "react-query";
import { vi, expect, describe, it } from "vitest";
import { createTestQueryClient } from "testHelpers/renderHelpers";
import { AuthProvider, useAuthContext } from "./AuthProvider";

const Wrapper: FC<PropsWithChildren> = ({ children }) => {
  return (
    <QueryClientProvider client={createTestQueryClient()}>
      <AuthProvider>{children}</AuthProvider>
    </QueryClientProvider>
  );
};

describe("useAuth", () => {
  it("throws an error if it is used outside of <AuthProvider />", () => {
    vi.spyOn(console, "error").mockImplementation(() => {});

    expect(() => {
      renderHook(() => useAuthContext());
    }).toThrow("useAuth should be used inside of <AuthProvider />");

    vi.restoreAllMocks();
  });

  it("returns AuthContextValue when used inside of <AuthProvider />", () => {
    expect(() => {
      renderHook(() => useAuthContext(), {
        wrapper: Wrapper,
      });
    }).not.toThrow();
  });
});
