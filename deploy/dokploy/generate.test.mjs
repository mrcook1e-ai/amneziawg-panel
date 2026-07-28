// @ts-check

import assert from "node:assert/strict";
import { copyFile, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { test } from "node:test";
import { checkImportArtifact } from "./generate.mjs";

const dokployDirectory = dirname(fileURLToPath(import.meta.url));

test("Given a divergent Compose probe when checked then it reports drift without rewriting the artifact", async () => {
  const temporaryDirectory = await mkdtemp(join(tmpdir(), "dokploy-generator-"));
  const composePath = join(temporaryDirectory, "docker-compose.yml");
  const configPath = join(temporaryDirectory, "template.toml");
  const artifactPath = join(temporaryDirectory, "import.base64");
  const probePath = join(temporaryDirectory, "probe-compose.yml");

  try {
    await Promise.all([
      copyFile(join(dokployDirectory, "docker-compose.yml"), composePath),
      copyFile(join(dokployDirectory, "template.toml"), configPath),
      copyFile(join(dokployDirectory, "import.base64"), artifactPath),
    ]);
    const originalArtifact = await readFile(artifactPath, "utf8");
    const canonicalCompose = await readFile(composePath, "utf8");
    await writeFile(probePath, `${canonicalCompose}# probe drift\n`, "utf8");

    await assert.rejects(
      checkImportArtifact({
        artifactPath,
        composePath,
        configPath,
        probeComposePath: probePath,
      }),
      /Compose drift detected/,
    );
    assert.equal(await readFile(artifactPath, "utf8"), originalArtifact);
  } finally {
    await rm(temporaryDirectory, { force: true, recursive: true });
  }
});
