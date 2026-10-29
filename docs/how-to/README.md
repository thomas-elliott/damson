# How-tos

Task guides for a running Damson. Each assumes a library with assets in it. If you have none, start at [`docs/getting-started.md`](../getting-started.md). Settings that use environment variables are in [`docs/configuration.md`](../configuration.md).

- [Answer "are we allowed to ship this?"](track-licences.md): licence detection, the four statuses, reviewing a licence so nothing overwrites it, and reading a project's licence risk.
- [Get Unity packages in, and see what is inside them](import-unity-packages.md): what extraction does with a `.unitypackage`, bulk-importing the local Asset Store cache with `dmsn sync`, browsing a pack, and cutting a subset version.
- [Pin a project to a version you have tested](pin-versions.md): how versions are ordered, which one is current, finding out when a pack has been updated upstream, and pinning a project.
- [Keep two teams' assets apart](separate-libraries.md): what a library separates and what it does not, creating one, switching, membership and roles.
- [Group assets you keep coming back to](collections.md): collections versus tags versus projects, and how to build and use one.

These describe Damson 0.3.0, which is pre-alpha. Where a feature has a gap, such as an action that exists only over the API or a setting with no control surface, the guide says so.
