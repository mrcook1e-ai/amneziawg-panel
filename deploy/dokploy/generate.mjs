// @ts-check

import { readFile, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const dokployDirectory = dirname(fileURLToPath(import.meta.url));

/**
 * @typedef {Readonly<{
 *   artifactPath?: string;
 *   composePath?: string;
 *   configPath?: string;
 *   probeComposePath?: string;
 * }>} ImportOptions
 */

/** @typedef {Readonly<{ compose: string; config: string }>} ImportPayload */

/** @typedef {Readonly<{ check: boolean; probeComposePath?: string }>} CliOptions */

/**
 * @param {ImportOptions} options
 * @returns {Readonly<{ artifactPath: string; composePath: string; configPath: string }>}
 */
function resolvePaths(options) {
  return {
    artifactPath: options.artifactPath ?? resolve(dokployDirectory, "import.base64"),
    composePath: options.composePath ?? resolve(dokployDirectory, "docker-compose.yml"),
    configPath: options.configPath ?? resolve(dokployDirectory, "template.toml"),
  };
}

/**
 * @param {ImportOptions} [options]
 * @returns {Promise<Readonly<{ payload: ImportPayload; encoded: string }>>}
 */
export async function createImport(options = {}) {
  const paths = resolvePaths(options);
  const [compose, config] = await Promise.all([
    readFile(paths.composePath, "utf8"),
    readFile(paths.configPath, "utf8"),
  ]);
  const payload = { compose, config };

  return {
    payload,
    encoded: Buffer.from(JSON.stringify(payload), "utf8").toString("base64"),
  };
}

/**
 * @param {ImportOptions} [options]
 * @returns {Promise<void>}
 */
export async function generateImportArtifact(options = {}) {
  const { artifactPath } = resolvePaths(options);
  const { encoded } = await createImport(options);
  await writeFile(artifactPath, `${encoded}\n`, "utf8");
}

/**
 * @param {ImportOptions} [options]
 * @returns {Promise<void>}
 */
export async function checkImportArtifact(options = {}) {
  const paths = resolvePaths(options);
  const { payload, encoded } = await createImport(options);

  if (options.probeComposePath !== undefined) {
    const probeCompose = await readFile(options.probeComposePath, "utf8");
    if (probeCompose !== payload.compose) {
      throw new Error(`Compose drift detected: ${options.probeComposePath}`);
    }
  }

  const actualArtifact = await readFile(paths.artifactPath, "utf8");
  const expectedArtifact = `${encoded}\n`;
  if (actualArtifact !== expectedArtifact) {
    throw new Error(`Generated import artifact is stale: ${paths.artifactPath}`);
  }
}

/**
 * @param {readonly string[]} argumentsList
 * @returns {CliOptions}
 */
function parseArguments(argumentsList) {
  /** @type {boolean} */
  let check = false;
  /** @type {string | undefined} */
  let probeComposePath;

  for (let index = 0; index < argumentsList.length; index += 1) {
    const argument = argumentsList[index];
    switch (argument) {
      case "--check":
        check = true;
        break;
      case "--compose": {
        const candidatePath = argumentsList[index + 1];
        if (candidatePath === undefined || candidatePath.startsWith("-")) {
          throw new Error("--compose requires a path");
        }
        probeComposePath = resolve(candidatePath);
        index += 1;
        break;
      }
      default:
        throw new Error(`Unknown argument: ${argument}`);
    }
  }

  if (!check && probeComposePath !== undefined) {
    throw new Error("--compose requires --check");
  }

  return probeComposePath === undefined ? { check } : { check, probeComposePath };
}

/**
 * @param {readonly string[]} argumentsList
 * @returns {Promise<void>}
 */
export async function main(argumentsList) {
  const options = parseArguments(argumentsList);
  if (options.check) {
    await checkImportArtifact({ probeComposePath: options.probeComposePath });
    return;
  }
  await generateImportArtifact();
}

const entrypoint = process.argv[1];
if (entrypoint !== undefined && resolve(entrypoint) === fileURLToPath(import.meta.url)) {
  main(process.argv.slice(2)).catch((error) => {
    const message = error instanceof Error ? error.message : "unknown error";
    console.error(`Dokploy import generation failed: ${message}`);
    process.exitCode = 1;
  });
}
