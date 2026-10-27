# Cutting a release.
#
# A release promotes current main to prod by pushing a vX.Y.Z tag, which
# triggers .github/workflows/release.yml: it runs no tests and promotes the
# main-<sha> image that CI already built and tested on main (no rebuild).
# Nothing is gated locally — run `just check` yourself first.
#
# A bare `just release` lists; a bare noun that cut a tag would be a surprise.

set shell := ["bash", "-euo", "pipefail", "-c"]

set default-list

# Tag a release — `just release tag v1.2.3`
[no-cd]
tag VERSION:
    #!/usr/bin/env bash
    set -euo pipefail

    [[ "{{ VERSION }}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] \
      || { echo "VERSION must be vX.Y.Z (for example v1.2.3)." >&2; exit 1; }
    [[ "$(git rev-parse --abbrev-ref HEAD)" == "main" ]] \
      || { echo "Releases are cut from main. Switch to main first." >&2; exit 1; }

    # main must be up to date with the remote (nothing unpushed, not behind).
    git fetch --quiet origin main
    if [[ "$(git rev-parse HEAD)" != "$(git rev-parse origin/main)" ]]; then
      echo "Error: local main is not in sync with origin/main — push/pull first." >&2
      exit 1
    fi
    SHA="$(git rev-parse origin/main)"

    if git ls-remote --exit-code --tags origin "refs/tags/{{ VERSION }}" >/dev/null 2>&1; then
      echo "Error: tag {{ VERSION }} already exists on origin." >&2
      exit 1
    fi

    # No local CI gate: the release workflow checks that main-<sha> exists
    # before retagging it, and CI only pushes that image once every test job
    # passes — so tagging a commit with no tested image fails safe.
    echo "Tagging {{ VERSION }} at $SHA ..."
    git tag -a "{{ VERSION }}" -m "{{ VERSION }}" "$SHA"
    git push origin "refs/tags/{{ VERSION }}"
    echo "Released {{ VERSION }}. The release workflow will promote main-$(echo "$SHA" | cut -c1-7) to {{ VERSION }}."

# Release the next patch version (vX.Y.Z+1) from the latest tag
[no-cd]
patch:
    just release::tag "$(just release::next patch)"

# Release the next minor version (vX.Y+1.0) from the latest tag
[no-cd]
minor:
    just release::tag "$(just release::next minor)"

# Release the next major version (vX+1.0.0) from the latest tag
[no-cd]
major:
    just release::tag "$(just release::next major)"

# The bump arithmetic, shared by the three above. A repo with no tag yet reads as
# v0.0.0, so a first patch/minor/major lands on v0.0.1 / v0.1.0 / v1.0.0.
[no-cd]
[private]
next LEVEL:
    #!/usr/bin/env bash
    set -euo pipefail
    git fetch --tags --quiet origin
    CUR="$(git tag -l 'v*' --sort=-version:refname | head -1)"
    IFS=. read -r MAJOR MINOR PATCH <<<"${CUR:-v0.0.0}"
    MAJOR="${MAJOR#v}"
    case "{{ LEVEL }}" in
      patch) PATCH=$((PATCH + 1)) ;;
      minor) MINOR=$((MINOR + 1)); PATCH=0 ;;
      major) MAJOR=$((MAJOR + 1)); MINOR=0; PATCH=0 ;;
    esac
    echo "v$MAJOR.$MINOR.$PATCH"
