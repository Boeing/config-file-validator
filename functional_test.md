# Functional Tests for the config-file-validator

Manual procedures for functionally testing the config-file-validator. These will eventually be used as requirements for creating automated function tests

## Setup

1. Build the latest changes in a container `docker build . -t cfv:<feature>` tagging the local container with a tag that indicates what feature is being tested
2. Run the container and mount the test directory `docker run -it --rm -v $(pwd)/test:/test --entrypoint sh cfv:<feature>`

## Basic Functionality

This section tests basic validator functionality

| Test | Expected Result | Notes |
| ---- | --------------- | ----- |
| `cd /test && /validator` | 37 passed and 12 failed | |
| `/validator /test` | 37 passed and 12 failed | |
| `/validator /test/fixtures/subdir /test/fixtures/subdir2` | 3 passed and 12 failed | |
| `/validator --help` | Help is displayed | |
| `/validator --version` | Output should read "validator version unknown" | |
| `/validator /badpath` | Error output should read: "An error occurred during CLI execution: Unable to find files: stat /badpath: no such file or directory" | |
| `/validator -v` | Error "flag provided but not defined: -v" is displayed on the terminal in addition to the help output | |


## Reports

This section validates report output

| Test | Expected Result | Notes |
| ---- | --------------- | ----- |
| `cd /test && /validator --reporter=json:-` | JSON output is produced on the terminal and summary is `"summary": {"passed": 37,"failed": 12}` | |
| `cd /test && /validator --reporter=standard:-` | Text output is produced on the screen | |
| `cd /test && /validator --reporter=junit:-` | JUnit XML is produced on the terminal | |
| `cd /test && /validator --reporter=sarif:-` | Sarif output is produced on the terminal | |
| `cd /test && /validator --reporter=json:/json_report.json` | JSON is written to `/json_report.json` and summary in the contents of the file reads `"summary": {"passed": 37,"failed": 12}` | |
| `cd /test && /validator --reporter=standard:/standard_report.txt` | Text is written to `/standard_report.txt` | |
| `cd /test && /validator --reporter=junit:/junit_report.xml` | JUnit XML is written to `/junit_report.xml` | |
| `cd /test && /validator --reporter=sarif:/sarif_report.sarif` | Sarif JSON is written to `/sarif_report.sarif` | This is currently failing as the sarif report is written to stdout in addition to the file |
| `cd /test && /validator --reporter=json:/json_report_2.json --reporter=standard:-` | JSON is written to `/json_report_2.json` and standard text is output to the terminal | |
| `cd /test && /validator --reporter=bad` | Error message "Wrong parameter value for reporter, only supports standard, json, junit, or sarif" should be displayed in addition to the help output | |
| `cd /test && /validator --reporter=json --quiet` | Nothing is displayed to the terminal since the `--quiet` flag suppresses the output | |
| `cd /test && /validator --reporter=json:/json_report_3.json --quiet` | Nothing is displayed to the terminal since the `--quiet` flag suppresses the output but the `/json_report_3.json` file is populated | |

## Grouping

This section validates organization of the output

| Test | Expected Result | Notes |
| ---- | --------------- | ----- |
| `cd /test && /validator --groupby=filetype` | Results are grouped by file type | |
| `cd /test && /validator --groupby=pass-fail` | Results are grouped by pass/fail | Bug filed for duplicate summaries |
| `cd /test && /validator --reporter=standard --groupby=pass-fail` | Results are grouped by pass/fail | Bug filed for duplicate summaries |
| `cd /test && /validator --groupby=directory` | Results are grouped by directory | |
| `cd /test && /validator --groupby=filetype,directory` | Results are grouped by file type, then directory | |
| `cd /test && /validator --groupby=pass-fail,filetype` | Results are grouped by pass-fail, then filetype | |
| `cd /test && /validator --groupby=pass-fail,directory` | Results are grouped by pass-fail, then directory | |
| `cd /test && /validator --groupby=pass-fail,directory,filetype` | Results are grouped by pass-fail, then directory, then file type | |
| `cd /test && /validator --groupby=pass-fail,pass-fail` | Error "Wrong parameter value for groupby, duplicate values are not allowed" is displayed with help output | |
| `cd /test && /validator --reporter=json --groupby=filetype,directory` | JSON Results are grouped by file type, then directory | This does not work, bug filed |
| `cd /test && /validator --reporter=junit --groupby=pass-fail` | Error "Wrong parameter value for reporter, groupby is only supported for standard and JSON reports" is displayed with help output | |
| `cd /test && /validator --reporter=sarif --groupby=directory` | Error "Wrong parameter value for reporter, groupby is only supported for standard and JSON reports" is displayed with help output | |

## Depth
| Test | Expected Result | Notes |
| ---- | --------------- | ----- |
| `cd / && /validator --depth=0 /test` | Nothing is displayed since there are no config files at the root of the `/test` directory and recursion is disabled with depth set to 0 | |
| `cd / && /validator --depth=1 /test` | Files in `/test/fixtures/` are validated | |
| `cd / && /validator --depth=2 /test` | Files in `/test/fixtures/*` directories are validated | |

## Globbing

| Test | Expected Result | Notes |
| ---- | --------------- | ----- |
| `cd /test && /validator --globbing "fixtures/**/*.json"` | All json files in subdirectories are displayed | |
| `cd /test && /validator --globbing "fixtures/**/*.json" /test/fixtures/subdir2` | All json files in subdirectories are displayed and other files in `/test/fixtures/subdir2` are displayed | |
| `cd /test && /validator --groupby=pass-fail --globbing "fixtures/**/*.json"` | All json files in subdirectories are displayed and grouped by pass/fail | |
| `cd /test && /validator --reporter=json --globbing "fixtures/**/*.json"` | All json files in subdirectories are displayed as a JSON report | |
| `cd /test && /validator "fixtures/**/*.json"` | Error "An error occurred during CLI execution: Unable to find files: stat fixtures/**/*.json: no such file or directory" should be displayed when using glob patterns without flag enabling it | |
| `cd /test && /validator --exclude-file-types=json --globbing "fixtures/**/*.json"` | Error "the -globbing flag cannot be used with --exclude-dirs or --exclude-file-types" is displayed | |
| `cd /test && /validator --exclude-dirs=subdir2 --globbing "fixtures/**/*.json"` | Error "the -globbing flag cannot be used with --exclude-dirs or --exclude-file-types" is displayed | |


## Environment Variables

Run `unset <var>` to unset the previous var before testing

| Test | Expected Result | Notes |
| ---- | --------------- | ----- |
| `export CFV_REPORTER=json:- && /validator /test` | JSON output should display on the terminal | |
| `export CFV_REPORTER=json:/test_env_var.json && /validator /test` | JSON output should be written to `/test_env_var.json` and NOT displayed on the terminal | |
| `export CFV_GLOBBING=true && cd /test && /validator "fixtures/**/*.json"` | Results should include json files in all subdirectories for fixtures | This does not work and a bug has been filed |
| `export CFV_QUIET=true && cd /test && /validator` | No output should be displayed on the terminal | |
| `export CFV_QUIET=false && cd /test && /validator` | Output should be displayed on the terminal | |
| `export CFV_GROUPBY=pass-fail && cd /test && /validator` | Output should be displayed on the terminal and grouped by pass-fail | |
| `export CFV_DEPTH=0 && cd /test && /validator` | Output should only display config files at the root of `/test/fixtures` | |
| `export CFV_EXCLUDE_DIRS=subdir2,subdir && cd /test && /validator` | `subdir` and `subdir2` directories should be excluded | |
| `export CFV_EXCLUDE_FILE_TYPES=yml,yaml,json,toml,properties,hocon,csv,hcl,ini,env,plist,editorconfig,xml  && cd /test && /validator` | No output since all types are excluded | |
| `export CFV_EXCLUDE_FILE_TYPES=yml,yaml,json,toml,properties,hocon,csv,hcl,ini,env,plist,editorconfig  && cd /test && /validator --exclude-file-types=""` | All config files should be displayed since the argument overrides the environment variable | |

## Exclude Dirs
| Test | Expected Result | Notes |
| ---- | --------------- | ----- |
| `cd /test && /validator --exclude-dirs=baddir` | Non-existent subdirectory is ignored | |
| `cd /test && /validator --exclude-dirs=subdir,subdir2` | `subdir` and `subdir2` directories are ignored | |
| ` cd /test && /validator --exclude-dirs=test /` | `test` subdirectory is excluded from root directory search path `/` | |


## Exclude File Types

| Test | Expected Result | Notes |
| ---- | --------------- | ----- |
| `cd /test && /validator --exclude-file-types=xml` | XML validation should be skipped | |
| `cd /test && /validator --exclude-file-types=yml` | `.yaml` and `.yml` should be excluded from validation | |
| `cd /test && /validator --exclude-file-types=YaML` | `.yaml` and `.yml` should be excluded from validation since argument values are not case sensitive | |


## Other Flags

| Test | Expected Result | Notes |
| ---- | --------------- | ----- |
| `cd /test && /validator --quiet` | Nothing is displayed to the terminal since the `--quiet` flag suppresses the output | |
| `cd /test && /validator /badpath --quiet` | Error "An error occurred during CLI execution: Unable to find files: stat /badpath: no such file or directory" is output to the terminal even through `--quiet` was enabled | |

