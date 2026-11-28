# Third-Party Notices

3m-ui is licensed under the GNU General Public License, version 3 (GPL-3.0).

3m-ui was relicensed from the Eclipse Public License 2.0 (EPL-2.0) to GPL-3.0: from commit `226c460` onward it is under GPL-3.0, while earlier commits and any already published releases remain under EPL-2.0, which cannot be revoked.

This repository and its release artifacts may include, download, invoke, or distribute third-party software that is subject to its own license terms. A third-party component's license is not changed by the license of 3m-ui.

## Mihomo Core

3m-ui uses the Mihomo Core as a separate core component. The Mihomo Core is distributed under the GNU General Public License, version 3 (GPLv3), according to the license applicable to the Mihomo version included with or downloaded by a particular 3m-ui release.

- Project: https://github.com/MetaCubeX/mihomo
- License: GNU General Public License v3.0
- License text: https://www.gnu.org/licenses/gpl-3.0.html

Mihomo is a separate program rather than a part of 3m-ui: 3m-ui downloads it and runs it as an independent process. When redistributing a 3m-ui release together with a Mihomo binary, the Mihomo binary and its corresponding source-code/license obligations remain subject to the GPLv3 terms applicable to that Mihomo release. This notice does not change those terms, and Mihomo's own copyright holders are unaffected.

## acme.sh

3m-ui's SSH management menu can drive [acme.sh](https://github.com/acmesh-official/acme.sh) to issue, deploy and renew TLS certificates. acme.sh is **not** bundled with, vendored into, or linked against 3m-ui: it is downloaded and installed as an independent tool on the operator's machine (by default `~/.acme.sh`) and invoked as a separate process by the menu.

- Project: https://github.com/acmesh-official/acme.sh
- License: GNU General Public License v3.0
- License text: https://www.gnu.org/licenses/gpl-3.0.html

Because acme.sh runs as a standalone executable and interacts with 3m-ui only through process invocation and files on disk, it is not a derivative work of 3m-ui and is not covered by 3m-ui's license. Where a 3m-ui release or installer gathers acme.sh, that component remains subject to GPLv3 and its license and attribution notices should be retained.

## License compatibility

3m-ui itself is licensed under GPL-3.0, the same license family as the Mihomo Core and acme.sh. Shipping these components together therefore raises no license-compatibility conflict, although each component keeps its own copyright holders and notices. A recipient's obligations under the GPL — including the requirement to make corresponding source available when distributing binaries — apply to each component according to its own license.

## Other dependencies

Dependencies and bundled components that are not part of 3m-ui itself remain subject to their respective upstream licenses. Where a release includes a third-party component, its applicable license and attribution notices should be retained.
