import { act, renderHook, waitFor } from "@testing-library/react";
import { vi , test, expect } from "vitest"
import * as API from "api/api";
import { MockTemplate } from "testHelpers/entities";
import { useDeletionDialogState } from "./useDeletionDialogState";

test("delete dialog starts closed", () => {
  const { result } = renderHook(() =>
    useDeletionDialogState(MockTemplate.id, vi.fn()),
  );
  expect(result.current.isDeleteDialogOpen).toBeFalsy();
});

test("confirm template deletion", async () => {
  const onDeleteTemplate = vi.fn();
  const { result } = renderHook(() =>
    useDeletionDialogState(MockTemplate.id, onDeleteTemplate),
  );

  //Open delete confirmation
  act(() => {
    result.current.openDeleteConfirmation();
  });
  expect(result.current.isDeleteDialogOpen).toBeTruthy();

  // Confirm delete
  vi.spyOn(API, "deleteTemplate");
  await act(async () => result.current.confirmDelete());
  await waitFor(() => expect(API.deleteTemplate).toBeCalledTimes(1));
  expect(onDeleteTemplate).toBeCalledTimes(1);
});

test("cancel template deletion", () => {
  const { result } = renderHook(() =>
    useDeletionDialogState(MockTemplate.id, vi.fn()),
  );

  //Open delete confirmation
  act(() => {
    result.current.openDeleteConfirmation();
  });
  expect(result.current.isDeleteDialogOpen).toBeTruthy();

  // Cancel deletion
  act(() => {
    result.current.cancelDeleteConfirmation();
  });
  expect(result.current.isDeleteDialogOpen).toBeFalsy();
});
