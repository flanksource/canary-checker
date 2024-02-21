# Contributing

Firstly, thank you for considering contributing to Canary-Checker! Here are some guidelines to help you get started.

## 📖 Code of Conduct

This project adheres to the CNCF Code of Conduct. By participating in this project, you agree to abide by its terms.

## Setup

Ensure that you have the latest version of Go installed on your machine.

1. Build the project by checking it out and running `make build` this will create a `.bin/canary-checker` binary

## Testing

1. Test an individual fixture using `canary-checker run fixtures/minimal/http_auth.yaml`, you can pass a directory or multiple files
1. Run a kubernetes based E2E test using `./test/e2e.sh` e.g. `fixtures/minimal`, this will spin up a kind cluster to run tests using a Kubernetes environment
    * Fixture folders can also include setup/teardown files:
      * `_setup.yaml` - Any kubernetes resources that need to be applied
      * `_setup.sh` - A bash script to run before the suite
      * `_post_setup.sh` - A bash script to run after test execution
1. Run a full operator test with a postgres DB: `./test/e2e-operator.sh`


## :bulb: Asking Questions

Always refer to the [docs](https://canarychecker.io/getting-started) before asking questions. You may create new issues for questions and help, just prefix the issue title with **Question:** or **Help:** and try and provide as much detail as possible.

## :inbox_tray: Opening an Issue

Before [creating an issue](https://help.github.com/en/github/managing-your-work-on-github/creating-an-issue), check if you are using the latest version of the project. If you are not up-to-date, see if updating fixes your issue first.

### :lock: Reporting Security Issues

Review our [Security Policy](https://github.com/flanksource/canary-checker?tab=security-ov-file). **Do not** file a public issue for security vulnerabilities.

## :love_letter: Feature Requests

Feature requests are welcome! While we will consider all requests, we cannot guarantee your request will be accepted. We want to avoid [feature creep](https://en.wikipedia.org/wiki/Feature_creep). Your idea may be great, but also out-of-scope for the project. If accepted, we cannot make any commitments regarding the timeline for implementation and release. However, you are welcome to submit a pull request to help!

- **Do not open a duplicate feature request.** Search for existing feature requests first. If you find your feature (or one very similar) previously requested, comment on that issue.

## Pull Requests

Pull requests are always welcome. To create a pull request, ensure that your changes are in a separate branch or fork that is based off `master`. When your changes are ready, submit your branch as a pull request against `master`.

- **Smaller is better.** Submit **one** pull request per bug fix or feature. A pull request should contain isolated changes pertaining to a single bug fix or feature implementation. **Do not** refactor or reformat code that is unrelated to your change. It is better to **submit many small pull requests** rather than a single large one. Enormous pull requests will take enormous amounts of time to review, or may be rejected altogether.

- **Prioritize understanding over cleverness.** Write code clearly and concisely. Remember that source code usually gets written once and read often. Ensure the code is clear to the reader. The purpose and logic should be obvious to a reasonably skilled developer, otherwise you should add a comment that explains it.

- **Comments should be used for why not what.** Many comments that merely explain what a piece of code is doing can be refactored away with better variable and function names and/or using shorter method.

- **Follow existing coding style and conventions.** Keep your code consistent with the style, formatting, and conventions in the rest of the code base. When possible, these will be enforced with a linter. Consistency makes it easier to review and modify in the future.

- **Include test coverage.** For new check types always include a passing (`_pass.yaml`) and failing (`_fail.yaml`) fixture


## :white_check_mark: Code Review

- **Review the code, not the author.** Look for and suggest improvements without disparaging or insulting the author. Provide actionable feedback and explain your reasoning.

- **You are not your code.** When your code is critiqued, questioned, or constructively criticized, remember that you are not your code. Do not take code review personally.

- **Always do your best.** No one writes bugs on purpose. Do your best, and learn from your mistakes.


## :pray: Credits

Adapted from [@jessesquires](https://github.com/jessesquires/.github/blob/main/CONTRIBUTING.md)
