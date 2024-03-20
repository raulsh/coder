import { expect, describe, it } from "vitest";
import { dispatchCustomEvent, isCustomEvent } from "./events";

describe("events", () => {
  describe("dispatchCustomEvent", () => {
    it("dispatch a custom event", () => {
      const eventDetail = { title: "Event title" };

      window.addEventListener("eventType", (event) => {
        if (isCustomEvent(event)) {
          expect(event.detail).toEqual(eventDetail);
        }
      });

      dispatchCustomEvent("eventType", eventDetail);
    });
  });
});
