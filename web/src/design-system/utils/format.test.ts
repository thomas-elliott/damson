import { describe, expect, it } from "vitest";

import { tidyAssetName } from "./format";

describe("tidyAssetName", () => {
  it("reads underscores as spaces", () => {
    expect(tidyAssetName("POLYGON_Fantasy_Village_URP")).toBe("POLYGON Fantasy Village URP");
    expect(tidyAssetName("__Rocks__and___Cliffs_")).toBe("Rocks and Cliffs");
  });

  it("leaves a name with nothing to tidy alone", () => {
    expect(tidyAssetName("Stylized Nature Pack v1.2")).toBe("Stylized Nature Pack v1.2");
    expect(tidyAssetName("sci-fi-crates")).toBe("sci-fi-crates");
  });

  it("keeps a name that is only underscores", () => {
    expect(tidyAssetName("___")).toBe("___");
  });
});
