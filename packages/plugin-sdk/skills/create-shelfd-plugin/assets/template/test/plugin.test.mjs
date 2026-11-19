import assert from "node:assert/strict";
import test from "node:test";
import { AuthExpiredError } from "@shelfd/plugin-sdk";
import { createStarterPlugin } from "../src/index.ts";

test("starter plugin throws AuthExpiredError when api_token is missing", async () => {
  const plugin = createStarterPlugin(async () => new Response("{}"));
  await assert.rejects(
    () =>
      plugin.catalog.browse(
        {},
        {
          userId: "u1",
          apiBase: "http://127.0.0.1:8080",
          invocationToken: "jwt",
          dataDir: "/tmp",
          config: {},
          stageBook: async () => {
            throw new Error("not expected");
          },
          patchConfig: async () => {},
          emitEvent: () => {},
          reportProgress: () => {},
          log: () => {},
        }
      ),
    (err) => err instanceof AuthExpiredError && err.code === -32001
  );
});
