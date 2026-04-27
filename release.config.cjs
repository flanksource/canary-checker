// Branches are selected dynamically to preserve the release workflow behaviour:
// - push to main/master creates beta pre-releases
// - manual workflow dispatch can create stable or rc releases
const fs = require('fs');

function getInputs() {
  if (!process.env.GITHUB_EVENT_PATH) {
    return {};
  }

  try {
    const event = JSON.parse(fs.readFileSync(process.env.GITHUB_EVENT_PATH, 'utf8'));
    return event.inputs || {};
  } catch (err) {
    console.warn(`Failed to read GitHub event inputs: ${err.message}`);
    return {};
  }
}

function getBranches() {
  const inputs = getInputs();

  if (process.env.GITHUB_EVENT_NAME === 'workflow_dispatch') {
    if (inputs.channel === 'stable') {
      return ['master'];
    }

    if (inputs.channel === 'rc') {
      return [{ name: 'master', channel: 'rc', prerelease: 'rc' }, { name: 'dummy-release' }];
    }
  }

  return [{ name: 'master', channel: 'beta', prerelease: 'beta' }, { name: 'dummy-release' }];
}

module.exports = {
  branches: getBranches(),
  plugins: [
    [
      '@semantic-release/commit-analyzer',
      {
        releaseRules: [
          { type: 'doc', scope: 'README', release: 'patch' },
          { type: 'fix', release: 'patch' },
          { type: 'chore', release: 'patch' },
          { type: 'refactor', release: 'patch' },
          { type: 'feat', release: 'patch' },
          { type: 'ci', release: false },
          { type: 'style', release: false },
          { type: 'major', release: 'major' },
        ],
        parserOpts: {
          noteKeywords: ['MAJOR RELEASE'],
        },
      },
    ],
    '@semantic-release/release-notes-generator',
    [
      '@semantic-release/github',
      {
        assets: [
          { path: './.bin/canary-checker-amd64', name: 'canary-checker-amd64' },
          { path: './.bin/canary-checker.exe', name: 'canary-checker.exe' },
          { path: './.bin/canary-checker_osx-amd64', name: 'canary-checker_osx-amd64' },
          { path: './.bin/canary-checker_osx-arm64', name: 'canary-checker_osx-arm64' },
          { path: './.bin/release.yaml', name: 'release.yaml' },
        ],
        // From: https://github.com/semantic-release/github/pull/487#issuecomment-1486298997
        successComment: false,
        failTitle: false,
      },
    ],
  ],
};
