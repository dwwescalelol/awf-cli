# Collaboration and Sharing

Looking to Go for inspiration in building a federated remote install service.

By leveraging git and git repository services like Bitbucket, GitHub and GitLab, awf can avoid the burden of hosting and maintaining a centralised remote repository. This gives the responsibility of version control, version tagging and permissioning to git.

awf aims to replicate the ability of language package managers (pip, cargo, npm) to install data from remote locations, with the interface of `awf install` as simple and minimal as possible. Publishing to a remote should be equally streamlined, using git as the backbone and tags for version tracking of individual documents. 

Every publishable document is atomic and depends on nothing, so there is no dependency graph to resolve, no version solver, and no propagation when a version is bumped.

## Commands

```
awf install github.com/org/repo foo@1.2.3
awf publish github.com/org/repo foo@1.2.3
```

## Problems

1. **Addressing.** One repo holds many documents, each versioned independently, but git tags are a single flat namespace per repo. The tag prefix is the document's path: `awf/<docType>/<id>/v<version>`. Resolution is then one `git ls-remote`, globbing `refs/tags/awf/*/<id>/v<version>`. Zero matches is not found, one match recovers the docType from the tag, and two or more is ambiguous and requires the user to qualify with `<docType>/<id>@<version>`.

2. **Remote structure.** The remote directory structure differs from the local one. Locally each version is an independent file for ease of use; on the remote that is expressed through tags instead. The remote also requires an assumed structure before anything can resolve against it. `awf remote init` creates it. Creating the repo itself, along with permissions and everything else, is up to the dev. The end user never needs to know any of this, since the CLI abstracts it away.

3. **Publishing.** `awf publish` is four git commands: write the file, commit, tag, push. Not an upload and not a PR. A published version must never change meaning, so publish checks the remote for the tag first and refuses if it already exists. Git already rejects an unforced push over an existing tag, so two people publishing the same version concurrently is safe. Concurrent publishes of different documents never collide, because their tag prefixes differ.

4. **Review.** A team sharing a repo needs review before a version becomes installable. Because a document is just a file, a change to it is an ordinary diff and an ordinary PR. Publishing splits in two: propose puts the change on a branch under `refs/heads/awf/propose/*`, and the tag is only created after merge. This is enforced with `git merge-base --is-ancestor`, which refuses to tag a commit that is not already on the default branch. Pure git, so it works identically on every host with no CI and no host API. Whether the default branch required approval is the host's branch protection, which is where that rule belongs.

5. **Provenance identity.** A version is a mutable label until its tag exists. Someone can run a document locally, then realise on publish that it needs a different version number. The sha is fixed at seal time and does not change, so run records stamp the sha and carry the version alongside it as decoration. A document with `sha: null` is unsealed and has no provenance.

6. **Availability and trust.** Using git as the backbone means there is no guaranteed availability and no shared trust anchor. Deleting a tag breaks anyone pinned to it, and each consumer verifies against the publisher's key rather than a central log. v1 accepts this: `seal` covers integrity and `git tag -s` covers authorship. Go solves the rest with a centralised proxy and checksum db, both opt-out. Both layers are additive, so they can be added later without changing addressing, publishing or review.

## Case study: Go

Go started as a 1-1 map from versioned package to repo, then added support for nested modules, making it many packages to one repo with each package versioned independently. That is effectively what awf is attempting here.

The Go community advises against nested modules, but every reason is dependency-driven: cross-module changes are not atomic, local development needs `replace` directives, and adding a `go.mod` silently carves a subtree out of its parent. Documents in awf are atomic and depend on nothing, so none of these criticisms apply.

## Case study: Docker

Docker separates the two ideas the same way. An image's identity is its digest, and a tag is a mutable pointer at a digest. Pulling by digest cannot be repointed; pulling by tag can. That is the same split as sha against version here.

Docker then adds what awf defers. A hash proves a file has not changed, but it does not prove who wrote it: anyone who can overwrite the file can recompute its hash to match. Docker closes that by having the publisher sign the image with a key only they hold. In awf that is `git tag -s`.

Docker also records where a built image came from: which source commit, which builder, which inputs. The image carries that record with it, so anyone holding the artifact can trace it back to what produced it. awf run does the same for its outputs, stamping the sha of the workflow and of every task that ran into the run record aswell as on any generaeted artifcat during the run.
