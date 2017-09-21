# Contributing

## Guidelines
### When to Search the Issue Tracker

If _any_ of the following apply:
- You believe you found a bug.  It might already be logged, or even have a fix underway!
- You have an idea for an enhancement or new feature.  Before you start coding away and submit a PR for your work, consult the issue tracker.
- You just want to find something to work on.

### When to Submit a New Issue

New issues may be submitted for enhancement requests as well as bug reports. However, we ask that you _please_ search first for similar existing issues to avoid posting a duplicate.

You are strongly advised to submit a new issue when you plan to perform work and submit a pull request (PR). See [When to Submit a Pull Request](#when-to-submit-a-pull-request) below.

A related matter of GitHub etiquette is when and how to post comments on Issues or PRs. Instead of simply posting "mee to! plus one", you can use the emoji responses to give a +1 or thumbs up.  Feel free to comment if you have more to add to the conversation. No one is going to scold you for adding details.

### When to Submit a Pull Request

Before submitting a PR, check the issue tracker for existing issues or relevant discussion. See what has been done, if anything. Perhaps there is good reason why certain changes have not already been made.

If the planned commits will involve significant effort on your part, you definitely want to either (1) submit a new issue, or (2) announce your intention to work on an existing issue. Why? Someone else could already be working on the problem. Also, there may be good reason why the change is not appropriate. The best way to check is to head to the issue tracker.

Only submit a PR once the indented edits have been either done or nearing completion.  It is OK to submit a PR with incomplete work if "WIP" or "Work in progress" is prefixed to the PR title prominently displayed in the description.

## How to Contribute
### Suggested Preparations

- Go language distribution - latest release or latest-1 (e.g. 1.8.3 and 1.9). [download](https://golang.org/doc/install)
- git client with command line support.  [download](https://git-scm.com/downloads)
- [GitHub](https://github.com/) account
- coffee, preferably black.  [some good stuff](http://haiticoffeeacademy.com/)

## Quick Start

1. Fork the repository on GitHub.  Just click the little Fork button at https://github.com/dcrdata/dcrdata
2. Clone your newly forked dcrdata repository

```
git clone git@github.com:my-user-name/dcrdata.git
```

3. Make a branch for your planned work, based on `master`

```
git checkout -b my-great-stuff master
```

4. Make edits. Review changes:
```
git status
```

5. Commit your work
```
git add -u
# don't forget to add that new file you made too
git add newfile.go
git commit # type a good commit message
```

6. 
