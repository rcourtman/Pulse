# [3.3.0](https://github.com/rcourtman/Pulse/compare/v3.2.2...v3.3.0) (2025-04-30)


### Bug Fixes

* **pbs:** Populate summary stats in processPbsTasks ([382cd87](https://github.com/rcourtman/Pulse/commit/382cd87896ac5d051fdbd6a02aa43aa371e2c3b9))
* prevent vertical scrollbar artifact on tabs nav ([483ed15](https://github.com/rcourtman/Pulse/commit/483ed15fb34806c55ab2f982b2ec508f32ea551e))
* **workflow:** remove non-existent label from prepare release PR creation ([af683bd](https://github.com/rcourtman/Pulse/commit/af683bdcc5ee31d608565effffd9b4a223952fe7))


### Features

* allow manual triggering of release workflow ([b4f668e](https://github.com/rcourtman/Pulse/commit/b4f668ee537a23a940151eaef016631171cd6971))
* Responsive layout improvements for header, tabs, and filters ([7073886](https://github.com/rcourtman/Pulse/commit/7073886d74648e468088f9a7d7b6f0730cfe2883))



## [3.2.2](https://github.com/rcourtman/Pulse/compare/v3.2.1...v3.2.2) (2025-04-30)


### Bug Fixes

* Add missing axios dependency ([cc0b800](https://github.com/rcourtman/Pulse/commit/cc0b8008fb1bfb29e4e0569d2dfeed6cef03677a))
* Add missing axios-retry dependency ([205e993](https://github.com/rcourtman/Pulse/commit/205e993bb8e3790df8d1e75246439bd028868d36))
* Correct assertions in config.test.js case 1 ([73f1c59](https://github.com/rcourtman/Pulse/commit/73f1c59f05abceb9f9d6047cc4515c567adaa9c8))
* Correct function name in config.test.js ([d26135c](https://github.com/rcourtman/Pulse/commit/d26135c0f5272090dfcb272dbf0e9404a0143a0f))
* Correct intentionally broken test case ([44aa0d7](https://github.com/rcourtman/Pulse/commit/44aa0d7e0cfd9834793eba43def785637f732bf2))
* Correct invalid tags in release workflow ([7d71f33](https://github.com/rcourtman/Pulse/commit/7d71f33b05ba3db60037b4abd73fb64dc1c66e21))
* **pbs:** Add missing worker types for verification tasks (fixes [#61](https://github.com/rcourtman/Pulse/issues/61)) ([3a1adfa](https://github.com/rcourtman/Pulse/commit/3a1adfa39cb47bc70f139142c12e3654e575e0c0))
* Re-center header logo on wider screens ([2177c45](https://github.com/rcourtman/Pulse/commit/2177c452f759417713f3a63bde36deb36866ba0f))
* Revert broken test, remove CI debug steps ([856622b](https://github.com/rcourtman/Pulse/commit/856622b24882cf6b578938bee4dbb768115b386c))


### Features

* Add test improvements and CI workflow ([3e037b3](https://github.com/rcourtman/Pulse/commit/3e037b33d67d0ecb0027ed4e61aac36478d1e637))
* Sync all local changes including UI and server updates ([cc482d3](https://github.com/rcourtman/Pulse/commit/cc482d36eb0d374aa4abb67036e3fb380570d630))


### Reverts

* Revert "chore: Bump version to v3.2.3" ([33adf44](https://github.com/rcourtman/Pulse/commit/33adf447739dfef0f7dab0673934bb3a152de4b1))



## [3.2.1](https://github.com/rcourtman/Pulse/compare/v3.2.0...v3.2.1) (2025-04-29)


### Bug Fixes

* Correct case statement syntax in install script ([62de6db](https://github.com/rcourtman/Pulse/commit/62de6db2afcbcc68e07b7164b698d34dbbd2dc51))
* Correct print_info function definition syntax ([3f07d4b](https://github.com/rcourtman/Pulse/commit/3f07d4b119a0b44f87436068c152535562e13649))
* Correct print_success function definition syntax ([cde72b4](https://github.com/rcourtman/Pulse/commit/cde72b47088aa8cb5cfaadab945014af805b6e18))
* Correct print_warning function definition syntax ([b867c31](https://github.com/rcourtman/Pulse/commit/b867c31aa8c34c973bfc39ca3382477907d39737))


### Features

* Await initial discovery cycle before server start ([0b71dbb](https://github.com/rcourtman/Pulse/commit/0b71dbbbdbbd46772f34e9e6465971ef56e53f6c))
* Enhance install script with env, npm ci, logs, pre-clone check ([aac31e8](https://github.com/rcourtman/Pulse/commit/aac31e8eb939fc08f0c973d4f0971928918e7cc2))
* **ui:** Improve PBS tab layout and health status ([6b6de57](https://github.com/rcourtman/Pulse/commit/6b6de575bc07d4cc70b1525fab4fb0ae49d375ba))



# [3.2.0](https://github.com/rcourtman/Pulse/compare/v3.1.5...v3.2.0) (2025-04-29)


### Bug Fixes

* Correct ID for list grouping filter in UI update ([7c4880c](https://github.com/rcourtman/Pulse/commit/7c4880c620dfe7353c813b720c267dc9566a592b))
* **pbs:** Resolve PBS data display issues and update docs - Corrects PBS API interaction logic, retains logging/fetch improvements, updates README/.env.example based on testing. ([4fed85d](https://github.com/rcourtman/Pulse/commit/4fed85dbf0b991327afd716dcd7626df1883d64b))
* Remove icon next to UI scale slider ([3ebcf38](https://github.com/rcourtman/Pulse/commit/3ebcf38617db322af972293c258221126fd1a34c))
* **ui:** Correct backup health status logic for recent snapshots - Marks as OK if snapshot <3d, even if no recent task reported. ([900b578](https://github.com/rcourtman/Pulse/commit/900b5787a646a58b14cc23e58a0bf7794b0e73d6))


### Features

* Add PBS docs, increase recent task limit, clean up logs ([3845b14](https://github.com/rcourtman/Pulse/commit/3845b140faf64587de9be28e1ca8e86c6fa4a377))
* Add UI scale slider ([593e899](https://github.com/rcourtman/Pulse/commit/593e8996f64d2b89565ff880d580775ffae51a92))
* Persist filter state in localStorage ([991a9d9](https://github.com/rcourtman/Pulse/commit/991a9d98b637efdb26785d2a23e704480c591068))
* **ui:** Improve PBS tab readability - Parses task targets, shortens UPIDs, updates GC status display. ([91c3db7](https://github.com/rcourtman/Pulse/commit/91c3db7f4b8c74c1dda55229fa0d09bf9484c512))



## [3.1.5](https://github.com/rcourtman/Pulse/compare/v3.1.4...v3.1.5) (2025-04-27)


### Bug Fixes

* **pbs:** Use correct colon separator for PBS API token auth ([d4e20dd](https://github.com/rcourtman/Pulse/commit/d4e20dddeeba24064ca8ba1bf9bcc9fa47750064))
* **pbs:** Use correct colon separator for PBS API token auth ([58e43cd](https://github.com/rcourtman/Pulse/commit/58e43cd391d72898ad23e49edc6ccbc62546b8e9))
* **storage:** Ensure PVE storage data is correctly aggregated ([4668477](https://github.com/rcourtman/Pulse/commit/4668477bcdbcf1c032e63ddd45e0e8e559ca9720))


### Features

* Grey out PBS/Backups tabs when PBS is unavailable ([5037df0](https://github.com/rcourtman/Pulse/commit/5037df0efe9bb4164ca6359c11f8e4751964d92e))



## [3.1.4](https://github.com/rcourtman/Pulse/compare/v3.1.3...v3.1.4) (2025-04-26)


### Bug Fixes

* Handle duplicate VMIDs and bump version to 3.1.4 ([3c0ceb9](https://github.com/rcourtman/Pulse/commit/3c0ceb9403865889d8f9ba8606a4bd62707926d6))
* Improve display of low I/O rates ([ffcd0c1](https://github.com/rcourtman/Pulse/commit/ffcd0c1ddafcf5ed74c553523ad5f586ece9552f))
* **install:** Force tag fetching during update to handle amended tags ([e8a2725](https://github.com/rcourtman/Pulse/commit/e8a27252008ffb7f705d8e545c21233e84fb48ca))



## [3.1.3](https://github.com/rcourtman/Pulse/compare/v3.1.2...v3.1.3) (2025-04-26)


### Bug Fixes

* Add safelist to tailwind config for dynamic classes ([aea444c](https://github.com/rcourtman/Pulse/commit/aea444c1fc04c0a9122c9ed49a72e5f3341d9ea5))
* Adjust build process for install script (no minify, install devDeps) ([6b1967c](https://github.com/rcourtman/Pulse/commit/6b1967c0a97b5eea5622749e8d3adb36587fde45))
* Align Dockerfile and server paths for LXC/Docker compatibility ([e1f7126](https://github.com/rcourtman/Pulse/commit/e1f7126e0dc73ae3ed61176c682e92745dca9e0e))
* Check in pre-built CSS and skip build in install script ([092c8ae](https://github.com/rcourtman/Pulse/commit/092c8ae7d3799d1330de7679deb1d93b67de4a62))
* Commit correct non-purged output.css for workaround ([aa70a87](https://github.com/rcourtman/Pulse/commit/aa70a87e9fafe0b84f77608cedc8daa0baf58bbb))
* Explicitly set NODE_ENV=development for build:css ([fe93c7d](https://github.com/rcourtman/Pulse/commit/fe93c7dc59b8f50399124eff32d5dc9dd571b2c2))
* Force build:css script to mirror dev:css (no watch) ([b270a22](https://github.com/rcourtman/Pulse/commit/b270a2259c3a61a1ee336f908e4acd94dff9ba65))
* **install:** Build CSS during install/update & use npx for TW v4 ([0271884](https://github.com/rcourtman/Pulse/commit/0271884b0173aeb308dca65f4578929b8587f2e9))
* **install:** Correct syntax error in print_error function ([f9e568f](https://github.com/rcourtman/Pulse/commit/f9e568f527e11375e6fd37408671b8b5c10a62e5))
* Resolve PBS banner bug and build issues (fixes [#55](https://github.com/rcourtman/Pulse/issues/55)) ([42dbf8e](https://github.com/rcourtman/Pulse/commit/42dbf8e2c7c3eec0deddd4547fdff17262e54a2a))
* Resolve UI loading issues for v3.1.2 ([70491c8](https://github.com/rcourtman/Pulse/commit/70491c88d2fd6a73fd2edb40af4bae6aa1d98fea))
* **server:** Add express static middleware and root route handler ([a93c9e7](https://github.com/rcourtman/Pulse/commit/a93c9e7da205ff678c6bf2dce00aa700c0f5c9d3))
* **server:** Define http server before initializing Socket.IO ([8af4121](https://github.com/rcourtman/Pulse/commit/8af41216560f5de4b6e34386b71c382a267d8d14))
* **server:** Remove obsolete code referencing globalPbsStatus ([45c5044](https://github.com/rcourtman/Pulse/commit/45c50441b808c79d1b603ba1a7af1ed2b07609f0))
* **server:** Resolve ReferenceErrors in /api/storage and socket requestData ([cfc9514](https://github.com/rcourtman/Pulse/commit/cfc9514ef0b22fb1ae156becf31caa3e3c57b951))
* **ui:** Make main table header alignment consistent ([#53](https://github.com/rcourtman/Pulse/issues/53)) ([58c3a1a](https://github.com/rcourtman/Pulse/commit/58c3a1ab20101d4d9b5e8e9881aa125347bfd1c3))
* **ui:** Standardize all table headers to left-align ([#53](https://github.com/rcourtman/Pulse/issues/53)) ([f2d83c2](https://github.com/rcourtman/Pulse/commit/f2d83c26a79607a1dcabcd374a5ad41f3fb9d10d))


### Features

* Downgrade to Tailwind v3 and restore build step ([2b11de9](https://github.com/rcourtman/Pulse/commit/2b11de93ca39a98a8af2207b0fed5d84ef15b9b1))
* **ui:** Display placeholder in PBS tab when not configured ([6fad187](https://github.com/rcourtman/Pulse/commit/6fad18709a2b5b386b1782ceef5ebb4d3ead6348))


### Reverts

* Remove pre-built CSS and restore build step ([df86e4a](https://github.com/rcourtman/Pulse/commit/df86e4a1dd633208ebf54ff58274665449e78628))



## [3.1.2](https://github.com/rcourtman/Pulse/compare/v3.1.1...v3.1.2) (2025-04-24)


### Features

* Report version tag in update script success message ([35a04cf](https://github.com/rcourtman/Pulse/commit/35a04cfc955471d08bddea7237d42d1e9cc71fec))
* **ui:** Display placeholder in PBS tab when not configured ([4c1f48d](https://github.com/rcourtman/Pulse/commit/4c1f48d6865c727a0c992d50ff4b87df501f0a79))



## [3.1.1](https://github.com/rcourtman/Pulse/compare/v3.1.0...v3.1.1) (2025-04-24)


### Bug Fixes

* Correct public directory path in Dockerfile ([f77fcdf](https://github.com/rcourtman/Pulse/commit/f77fcdf95aec00a44404de121ce156a4510e1a83))



# [3.1.0](https://github.com/rcourtman/Pulse/compare/v3.0.2...v3.1.0) (2025-04-24)


### Bug Fixes

* Correct paths after frontend asset refactor ([665b92b](https://github.com/rcourtman/Pulse/commit/665b92b3779bd18524aa96914be32a55ab9eacc6))
* **debug:** Place guestId_temp inside deep copy ([3b65aaa](https://github.com/rcourtman/Pulse/commit/3b65aaab5d57255bf0970cf34de371a9275bc58e))
* Ensure correct PBS status is emitted to frontend ([5a83841](https://github.com/rcourtman/Pulse/commit/5a8384116c733fb2f66fe7ff3aa524a6c029fe18))
* **frontend:** Prevent metrics update overwriting PBS status ([a8e79c9](https://github.com/rcourtman/Pulse/commit/a8e79c9c1c80e0e8e069a33200e703319db7a72b))
* Handle 'N/A' string in PBS GC status display ([4e2d99d](https://github.com/rcourtman/Pulse/commit/4e2d99d1bd0edde13791875120f4909ed625905f))
* Remove node status debug log ([66aa670](https://github.com/rcourtman/Pulse/commit/66aa670ce028bcec17e33e97932ce1bb8cf6e32e))
* Resolve Tailwind content path issue after refactor ([7e174e7](https://github.com/rcourtman/Pulse/commit/7e174e78ce8495ede68765b27f43aa7451160c5e))
* Restore rate calc and improve history reset logic ([632a500](https://github.com/rcourtman/Pulse/commit/632a5004e4eff0f60430c2cbf1fedb2d9459b2be))
* simplify PBS connected status text ([a89b56d](https://github.com/rcourtman/Pulse/commit/a89b56dd138f10486da72b5a22d5a4adf87e96dd))
* **ui:** correct sort state key handling in updateSortUI for PBS tables ([459d2f7](https://github.com/rcourtman/Pulse/commit/459d2f71a7326e0a22fb2acdac974b152f885ad5))
* **ui:** improve PBS table header contrast in dark mode ([99678d3](https://github.com/rcourtman/Pulse/commit/99678d3a2a488ab43f7a3a87aafb7784cddb9d17))
* **ui:** prioritize explicit key in updateSortUI to avoid check failure ([4b1abe9](https://github.com/rcourtman/Pulse/commit/4b1abe957b8de2f4c61c4609eb31db5c18a9595b))
* **ui:** remove initial PBS loading message correctly ([14439c3](https://github.com/rcourtman/Pulse/commit/14439c36ca316badfed363722b308499ad8be734))
* **ui:** resolve hoisting and reference errors in PBS/sort logic ([a19dffb](https://github.com/rcourtman/Pulse/commit/a19dffbccb6332f95732b5833bad8d39e8bf31cb))
* Use deep copy for metrics data point ([e571d73](https://github.com/rcourtman/Pulse/commit/e571d73369961bb47eeb909f98af56dc192f3913))
* Use explicit copy for metrics data point to prevent stale disk values ([b2b6f48](https://github.com/rcourtman/Pulse/commit/b2b6f48dd9bccea5dfdae005d241cf2d2f6105f0))
* Use maxcpu from /nodes endpoint and remove debug logs ([9c1f461](https://github.com/rcourtman/Pulse/commit/9c1f4618cf18c67b5485b0c0f04d693c6d6fc68f))


### Features

* Add debug log for node status before merge ([be1c6a8](https://github.com/rcourtman/Pulse/commit/be1c6a8fef6dc4d38d158ef7839421e33997876d))
* add icons to navigation tabs ([5f76260](https://github.com/rcourtman/Pulse/commit/5f76260c4132c8200ddc4a8ca267a883ba8c4f45))
* Add specific debug log for maxcpu ([30b96bd](https://github.com/rcourtman/Pulse/commit/30b96bd33ad8eeda131b488726da369795deb256))
* Re-add specific debug log for maxcpu ([e09549d](https://github.com/rcourtman/Pulse/commit/e09549d3e75414708f384962a26a22ffb947a30c))


### Reverts

* Remove potentially problematic maxcpu debug log ([68a41c7](https://github.com/rcourtman/Pulse/commit/68a41c7b2b78ea5d87c3203c589dc8e44b46f66f))



## [3.0.2](https://github.com/rcourtman/Pulse/compare/v3.0.1...v3.0.2) (2025-04-24)



## [3.0.1](https://github.com/rcourtman/Pulse/compare/v3.0.0...v3.0.1) (2025-04-24)


### Bug Fixes

* Ensure systemd service is configured during update ([afe00ac](https://github.com/rcourtman/Pulse/commit/afe00ac6843bca943db49d54ca71b3dcd2dbf384))
* **script:** Improve service cleanup and fix final instructions ([30f6571](https://github.com/rcourtman/Pulse/commit/30f6571af4a077c960d6524563d88d9569629d93))
* **script:** Load .env file via systemd EnvironmentFile ([5dfac85](https://github.com/rcourtman/Pulse/commit/5dfac8582de9d17a42a8cfbfb4d68eb7fbcf4d85))
* **script:** Prevent git clean from removing ignored .env file ([d7539ab](https://github.com/rcourtman/Pulse/commit/d7539ab43dfc1dd6e147e3d6d81663c5bf3a893b))
* **script:** Update .env file paths in configure_environment ([526a6e8](https://github.com/rcourtman/Pulse/commit/526a6e87f54659d55a83c5c9b61386e086802328))
* **script:** Use explicit path for EnvironmentFile in systemd ([b8aa564](https://github.com/rcourtman/Pulse/commit/b8aa56490677f7349cdd226f7601bceb5c227cc7))
* Use git reset --hard for robust script updates ([78fa07e](https://github.com/rcourtman/Pulse/commit/78fa07e8f181ec15640fcb8b9ece6667227e8fbf))


### Features

* Configure Tailwind CSS for class-based dark mode ([233f8e5](https://github.com/rcourtman/Pulse/commit/233f8e5bb5f15e71accc169f31e2b2208f437108))



# [3.0.0](https://github.com/rcourtman/Pulse/compare/v2.5.0...v3.0.0) (2025-04-23)



# [2.5.0](https://github.com/rcourtman/Pulse/compare/v1.5.1...v2.5.0) (2025-04-23)


### Bug Fixes

* Add rsync to dependencies for update function ([655c6e8](https://github.com/rcourtman/Pulse/commit/655c6e8d24e00f7c830c9642634b2458bbcceedc))
* Allow script to continue after interactive update for cron prompt ([3b62e4b](https://github.com/rcourtman/Pulse/commit/3b62e4bbe27af9cd26d6834dd40c5f390754159f))
* **app:** remove sorting setup for non-existent tables ([d5aeccf](https://github.com/rcourtman/Pulse/commit/d5aeccfc05e694d11e83ec2c83d7cb6972adeaff))
* correct network rate calculation to properly handle Proxmox cumulative counters ([aa67bd3](https://github.com/rcourtman/Pulse/commit/aa67bd3b464b6c0de1ce49f68994125bd6d8b382))
* Correct parsing for pvesm list and pveam available ([35b93f5](https://github.com/rcourtman/Pulse/commit/35b93f512fc87b2cc6fb5781051fef774b5bcd68))
* **docs:** Correct community script curl command ([8742366](https://github.com/rcourtman/Pulse/commit/874236661294223e24fdce0dccda3dd26904cc0d))
* Escape parentheses in display_suffix assignments ([f57659e](https://github.com/rcourtman/Pulse/commit/f57659e4a118796bf6f61fd67edbd166e17a8d4c))
* improve NetworkTableHeader robustness and drag-and-drop behavior ([6159b33](https://github.com/rcourtman/Pulse/commit/6159b33c91ebf369aaedbce79086e38d0fae940a))
* Improve update robustness in install script ([c0e4dab](https://github.com/rcourtman/Pulse/commit/c0e4dabe44072d5e2d9e3447c805b77304d385c7))
* **install:** always prompt for auto-update if no cron job found, even if crontab is empty ([63bc98d](https://github.com/rcourtman/Pulse/commit/63bc98d67a774a7a4211c441a46fca2acb47ab58))
* **install:** always prompt for cron setup at end of script, add debug print ([635a660](https://github.com/rcourtman/Pulse/commit/635a660d0312a307de6b1dbcf95f1e343aa28f69))
* **install:** show actual cron command and comment in schedule prompt ([33c0738](https://github.com/rcourtman/Pulse/commit/33c07380b0b5c93dad24bce1e7239a6e5e7c69a9))
* **mock:** update mock data cleanup to preserve server-side settings ([054796d](https://github.com/rcourtman/Pulse/commit/054796d68f92cf484ab5cfb999f45c460a41dc39))
* Prevent download attempt if template support is unknown (jq missing) ([aa2800e](https://github.com/rcourtman/Pulse/commit/aa2800ef50550b558164e8d166a196e26667554d))
* Prevent set -e exit on grep in prompt_for_cron_setup ([c2f626a](https://github.com/rcourtman/Pulse/commit/c2f626a693da63d9cec1d8f472e028310a06f393))
* prevent storage header transparency in dark mode ([6be0c78](https://github.com/rcourtman/Pulse/commit/6be0c78b7407bb8c06b7354e2533f153fc0da8ba))
* Reliably determine script path for cron setup ([bdd2eda](https://github.com/rcourtman/Pulse/commit/bdd2edad13b6f1cc276b34245ad4e90ffb21155b))
* Rely only on boolean check for template support ([55f5be6](https://github.com/rcourtman/Pulse/commit/55f5be6a43c48527779a8bbb24504e8d1369b55e))
* Remove -P flag from pvesm status to avoid arg error ([de3f957](https://github.com/rcourtman/Pulse/commit/de3f95759ea56c296260c52073a3724087c353ec))
* Remove parentheses from fallback storage suffix ([05fb38f](https://github.com/rcourtman/Pulse/commit/05fb38fbea2705f11333678d7c67becd021f174d))
* Remove parentheses from storage display suffix ([4afd47a](https://github.com/rcourtman/Pulse/commit/4afd47ae25c73056d627628ac27f49d2b2c54040))
* Replace backticks with single quotes in jq warning ([97055ea](https://github.com/rcourtman/Pulse/commit/97055ea18c403de1d322c500780502f03ebdebf1))
* Reset repo before update to avoid conflicts ([788972f](https://github.com/rcourtman/Pulse/commit/788972fca75c3dea590529f63a12c394fb4156f9))
* resolve security vulnerabilities by removing unused packages ([ce4d3f1](https://github.com/rcourtman/Pulse/commit/ce4d3f1a5ff94f4678c6d7e1e0d839894a6a6018))
* **script:** Add gpg dependency for NodeSource key import ([9ecb180](https://github.com/rcourtman/Pulse/commit/9ecb18061f60ca6e43f33c928b5ebd66cc113d6e))
* **script:** Run git pull as pulse user during update ([ff8ec07](https://github.com/rcourtman/Pulse/commit/ff8ec078171934e0da907c0c75d5d23d7623e0c2))
* Suppress gpg overwrite prompt ([2db4bf5](https://github.com/rcourtman/Pulse/commit/2db4bf59b8c7a1d73fa427bc479b12a871a98512))
* Update .gitignore and untrack server/node_modules ([cd187a3](https://github.com/rcourtman/Pulse/commit/cd187a308488eabff2a27978bacd64b5013dcdea))
* update API proxy port to 7654 ([18e5687](https://github.com/rcourtman/Pulse/commit/18e56873d53b57a0615f7ba46304412deeb9dd7b))
* update Docker Compose environment variables to match .env.example format ([7108b7b](https://github.com/rcourtman/Pulse/commit/7108b7bfb7ae537b2e422f487564506c687a6396))
* Update repository URL in script and README ([c3656ae](https://github.com/rcourtman/Pulse/commit/c3656ae56fd89e06299bb6cca5a46ae5c1f93bc1))
* Use pvesh and jq for more robust storage detection ([28ca746](https://github.com/rcourtman/Pulse/commit/28ca7469e87d4bb275b3b1f0fb564d60ed320780))
* Use pvesm status as simpler jq fallback ([d63cad7](https://github.com/rcourtman/Pulse/commit/d63cad74cbd00b854d0e63194ce9dd76df743820))


### Features

* Add API retry logic ([#51](https://github.com/rcourtman/Pulse/issues/51)) and sort persistence ([#50](https://github.com/rcourtman/Pulse/issues/50)) ([f7acc6e](https://github.com/rcourtman/Pulse/commit/f7acc6ea98ab3634edfd49d79f70fa52f806abf5))
* Add automatic LXC template download if needed ([fc0517d](https://github.com/rcourtman/Pulse/commit/fc0517d5f6113d5d79e78c524e2090835d85a31e))
* Add automatic update option via cron ([d8922ef](https://github.com/rcourtman/Pulse/commit/d8922efffc53672b6a078f8c8bd3dd5303bba117))
* Add debug logging to prompt_yes_no ([e14e3c0](https://github.com/rcourtman/Pulse/commit/e14e3c0a952b5b4f3aac40a95979049f7f66bb75))
* add dev-real environment configuration for non-mock data development ([fc9f1db](https://github.com/rcourtman/Pulse/commit/fc9f1db52d95649553872a53080296c62d7ad106))
* Add Dockerfile and Docker Compose setup ([14bf3b1](https://github.com/rcourtman/Pulse/commit/14bf3b18d920cb08ec181252e673990784156941))
* Add missing logo file from main branch ([e66dd30](https://github.com/rcourtman/Pulse/commit/e66dd3069577b9a3b915823e87927f8c0e7d97b0))
* add mock data cleanup utilities ([c6b06da](https://github.com/rcourtman/Pulse/commit/c6b06da8b14969727a5e2be547009c87c30fc21b))
* Add multi-endpoint monitoring support (v2.5.0) ([989f784](https://github.com/rcourtman/Pulse/commit/989f784f7be976d126a339333977c282421e65dc))
* Add script to automate LXC creation and Pulse installation ([83613b5](https://github.com/rcourtman/Pulse/commit/83613b5fb0027db755cbb99a0b6eb45854a7e048))
* Add version display to frontend ([2f77e8f](https://github.com/rcourtman/Pulse/commit/2f77e8f26d9b6ec67d0782d32758750f2a5aad3e))
* Allow UI embedding in iframes (closes [#10](https://github.com/rcourtman/Pulse/issues/10)) ([7580930](https://github.com/rcourtman/Pulse/commit/758093079a23a993d55c873755ece08905f84bf0))
* **app:** enhance UI and add cluster detection ([02e4070](https://github.com/rcourtman/Pulse/commit/02e40701417412616effa235a36ed5ef4ec26781))
* Apply UI/UX and performance improvements ([bec137d](https://github.com/rcourtman/Pulse/commit/bec137db068a54e441f49ffcfd6d0f01c3e2ed00))
* expose mock data environment variables to frontend ([d2ffc1d](https://github.com/rcourtman/Pulse/commit/d2ffc1dd356f9828523bac8acdc9246a267ddd9c))
* Focus search on keypress, blur on Enter/Escape ([38e2684](https://github.com/rcourtman/Pulse/commit/38e26843242e737f32a594275bc9a96ad72be6d6))
* improve mock data handling and add demo mode banner ([960eb27](https://github.com/rcourtman/Pulse/commit/960eb27ee2cc7d8c344dd086e42fd5c51777d530))
* Improve storage selection and template download logic ([408347c](https://github.com/rcourtman/Pulse/commit/408347c328f728a8c96f4c326ea84a91c1d778f0))
* Improve update stability and fix server dependencies ([cc20e77](https://github.com/rcourtman/Pulse/commit/cc20e77fea90d0beffa86dc96734eedc793d87cd))
* **install:** Add URL validation and update prompt defaults ([c06e546](https://github.com/rcourtman/Pulse/commit/c06e54681b035400053488cc4696adbb3f51e38d))
* **install:** clarify .env overwrite and cron schedule prompts for user safety ([0313a2f](https://github.com/rcourtman/Pulse/commit/0313a2f62031f692aa65413d5c3752be5f2e00cd))
* **install:** full script refactor with atomic update, backup, health check, and UX improvements ([ff7f1f0](https://github.com/rcourtman/Pulse/commit/ff7f1f03b860855de7cf37cf10ab36ccdee5736b))
* **install:** improve .env overwrite prompt with diff and explicit warning ([c03f941](https://github.com/rcourtman/Pulse/commit/c03f941700979cfc0aabbedd775d635df465cced))
* **install:** improve cron setup prompt in install script Check for existing cron job during install and provide options to change, remove, or keep the existing schedule. ([533a970](https://github.com/rcourtman/Pulse/commit/533a970a80102124d7ee31d9b01df81cd5efe7cc))
* **install:** make .env overwrite prompt friendlier and less alarming for users ([6d11f55](https://github.com/rcourtman/Pulse/commit/6d11f554a500092e27e614d7f84896b3d774aa22))
* Link footer version to GitHub releases ([13aba29](https://github.com/rcourtman/Pulse/commit/13aba2975c7d4eec8085a438e51a0b608cdb1846))
* **metrics:** improve network traffic simulation with realistic patterns ([c7f6784](https://github.com/rcourtman/Pulse/commit/c7f6784c08fd8569d042dda706f70399b4f47293))
* **mock:** enhance mock client with HA support ([18f8673](https://github.com/rcourtman/Pulse/commit/18f86738d4ac610beec407d63666c07a6d50548e))
* **mock:** enhance mock server with HA support ([395d0c3](https://github.com/rcourtman/Pulse/commit/395d0c3d6dde45c8bf7ecb7a342b3087ed0e9d4d))
* **mock:** update custom mock data with HA states ([256bf79](https://github.com/rcourtman/Pulse/commit/256bf790c470f6f72814f81d0cafe01a08134da2))
* **network:** add cluster detection and improve column visibility ([f55097d](https://github.com/rcourtman/Pulse/commit/f55097d71874371879f09c45b6bc2414553b83d6))
* **network:** enhance data processing with detailed logging ([4c028e8](https://github.com/rcourtman/Pulse/commit/4c028e8a44376735c714be7a8130e7c38c9ded94))
* **network:** enhance filtering and column management ([9e2c1b6](https://github.com/rcourtman/Pulse/commit/9e2c1b60e79885a7bc4e7b1c0ef7bb1cb3b54cb3))
* **network:** enhance guest data handling and network traffic patterns ([9555d87](https://github.com/rcourtman/Pulse/commit/9555d878886f6c05e6b382a87c2eb09db604cb0a))
* **network:** enhance HA status display and filtering ([429a929](https://github.com/rcourtman/Pulse/commit/429a929fc9ce396713ec4d19839056fac4d4a190))
* **network:** update default column configuration ([1f5d63e](https://github.com/rcourtman/Pulse/commit/1f5d63eba85bf896e6444bfb8fa82325c80d9a4a))
* Replace codebase entirely with simplified version ([83885e3](https://github.com/rcourtman/Pulse/commit/83885e310460cafee1377d0636fd375b384b3896))
* **script:** Add PVE CLI commands for API token generation ([10973e8](https://github.com/rcourtman/Pulse/commit/10973e807fc98d17a290fe4d1e8e04b64f6e8b3d))
* **script:** Add update/remove options to install script ([4613baa](https://github.com/rcourtman/Pulse/commit/4613baaf20f704b7f51f37aa8f5ac0a62532c8ba))
* **search:** enhance column filtering with role support ([a31c610](https://github.com/rcourtman/Pulse/commit/a31c610bf054e6a3deb777dab72ee3ae634be7d2))
* **search:** enhance network search with role filtering and improved metric operators ([654f497](https://github.com/rcourtman/Pulse/commit/654f4973ede64e2ebab37ed1a0be1a00e05d5091))
* **search:** enhance search field with role filters and improved UI ([65071e7](https://github.com/rcourtman/Pulse/commit/65071e71decca1db73184c2fd30ca8aa86fd5ec9))
* **server:** enhance node manager with HA support ([5ac51db](https://github.com/rcourtman/Pulse/commit/5ac51db0a8a5dcfb5de649f213338c1dee97eb07))
* **server:** update config and types with HA support ([6d514d7](https://github.com/rcourtman/Pulse/commit/6d514d7bbbbe6b19520074c782c1a8e432badb77))
* **settings:** enhance column management and user settings ([28e033d](https://github.com/rcourtman/Pulse/commit/28e033da9dd48583f525170df847fb52fb5c6086))
* **socket:** improve socket handling and mock data generation ([090565f](https://github.com/rcourtman/Pulse/commit/090565f71f7e021a1cdfb58529fa3e1129d9c579))
* **ui:** add NodeSelect component for node filtering ([0d7f1bd](https://github.com/rcourtman/Pulse/commit/0d7f1bd76317bc9a73b4943f4880fe3ca489b563))
* Update dashboard screenshot ([e708b9a](https://github.com/rcourtman/Pulse/commit/e708b9aeb944c2e0b41f1b91515160c17ee6f78a))
* Update dashboard screenshot again ([8e2c0fc](https://github.com/rcourtman/Pulse/commit/8e2c0fc43ce084f11ec1bf123e92ec760664806f))
* Update version link to point to specific release tag ([99a27cd](https://github.com/rcourtman/Pulse/commit/99a27cd2239b4eaa88e6c47584b14d5e88736a52))


### Performance Improvements

* Only poll Proxmox API when clients are connected ([0a67199](https://github.com/rcourtman/Pulse/commit/0a67199e6770b6213813e1d3189301e5190e051b))


### Reverts

* **install:** Revert install-pulse.sh to pre-April 23rd state (bdd2eda) ([5f94d99](https://github.com/rcourtman/Pulse/commit/5f94d99dafeda730bd5817477085fa6e3e9e3d07))



## [1.5.1](https://github.com/rcourtman/Pulse/compare/v1.5.0...v1.5.1) (2025-03-10)



# [1.5.0](https://github.com/rcourtman/Pulse/compare/v1.4.1...v1.5.0) (2025-03-10)


### Bug Fixes

* add strictPort option to Vite config to prevent port incrementing ([4ab7cd1](https://github.com/rcourtman/Pulse/commit/4ab7cd1fe979c958433a08987f42f58552b84de1))
* include public directory in Docker build for logo assets ([745d452](https://github.com/rcourtman/Pulse/commit/745d452db05506ad6bb55a767d5f349e0e2718c9))
* prevent Vite from incrementing ports when default port is in use ([b6353f6](https://github.com/rcourtman/Pulse/commit/b6353f609e23d28edf220a48f05902c4020a4790))


### Features

* update logo implementation to use generated logo with pulse animation ([1c572aa](https://github.com/rcourtman/Pulse/commit/1c572aaf5c26f597d50e3f2a2d1b2a7f285abc17))



## [1.4.1](https://github.com/rcourtman/Pulse/compare/v1.4.0...v1.4.1) (2025-03-10)


### Bug Fixes

* add RequestHandler type to all API routes ([c7be5bb](https://github.com/rcourtman/Pulse/commit/c7be5bb1a053754fd5a66d8a04a86f959ba97971))
* static file serving in production Docker container ([0bdbbc6](https://github.com/rcourtman/Pulse/commit/0bdbbc6258709ecddabb0e50ee3136ee570f22d0))



# [1.4.0](https://github.com/rcourtman/Pulse/compare/v1.3.1...v1.4.0) (2025-03-10)


### Bug Fixes

* update frontend version display to 1.3.1 ([cf43830](https://github.com/rcourtman/Pulse/commit/cf438300d47d65aacab2854091e74c9712771e28))
* update package-lock.json versions to 1.3.1 ([8b0b2cc](https://github.com/rcourtman/Pulse/commit/8b0b2cc820e2b3df71a0594ee5984102324e230f))


### Features

* improve favicon support with multiple sizes and formats ([929497a](https://github.com/rcourtman/Pulse/commit/929497a1e495825eb3d223eaa37a9a0f5c61e04d))



## [1.3.1](https://github.com/rcourtman/Pulse/compare/v1.3.0...v1.3.1) (2025-03-10)


### Bug Fixes

* update axios to 1.7.4 to address SSRF vulnerability (CVE-2024-39338) ([fbe0730](https://github.com/rcourtman/Pulse/commit/fbe07309c368a2078859ac8da56dabeeb3538671))
* update axios to 1.8.2 to address SSRF and Credential Leakage vulnerability (GHSA-jr5f-v2jv-69x6) ([961386f](https://github.com/rcourtman/Pulse/commit/961386f24c3006656987ab70871638f9223f425c))



# [1.3.0](https://github.com/rcourtman/Pulse/compare/v1.2.1...v1.3.0) (2025-03-09)


### Bug Fixes

* ensure 'No Matching Systems' message spans full table width ([763105c](https://github.com/rcourtman/Pulse/commit/763105c478e3cf72bfa4c32742834363b128e58c))
* improve dropdown behavior and styling - remove focus outline and fix blur behavior ([ee7a2bc](https://github.com/rcourtman/Pulse/commit/ee7a2bc050f4292f9a44c4e30d8a60d525eef903))


### Features

* add tooltip to filter icon button ([0d29695](https://github.com/rcourtman/Pulse/commit/0d29695e5c7ea9ff9ae87b5a3933fc68993ee72a))
* clarify node vs guest counts in dropdown by adding descriptive labels ([e1c6e22](https://github.com/rcourtman/Pulse/commit/e1c6e22a7d9ef4bc987ab8f2642c6a98da712d90))
* enhance export functionality with additional fields (Type, ID, Uptime) in NetworkDisplay ([a00a8d5](https://github.com/rcourtman/Pulse/commit/a00a8d52cd8b29f165ea438880107fec7143019b))
* remove icons from column headers to save space ([b4e6e26](https://github.com/rcourtman/Pulse/commit/b4e6e26e0193ed15c76add772ff179f395c81dc4))



## [1.2.1](https://github.com/rcourtman/Pulse/compare/v1.2.0...v1.2.1) (2025-03-04)



# [1.2.0](https://github.com/rcourtman/Pulse/compare/v1.1.1...v1.2.0) (2025-03-04)


### Features

* increase uptime column width to prevent sort icon overlap ([3f90af1](https://github.com/rcourtman/Pulse/commit/3f90af11dbbe0599001cdb9794343b7fceb0b8bd))
* prevent text selection in table header cells for better UX ([cf39adb](https://github.com/rcourtman/Pulse/commit/cf39adbfc6229579cd784a05858e94d8aec80756))



## [1.1.1](https://github.com/rcourtman/Pulse/compare/v1.1.0...v1.1.1) (2025-03-04)


### Bug Fixes

* handle undefined/null values in network rate formatting to prevent 'undefined/s' display ([cfa84ca](https://github.com/rcourtman/Pulse/commit/cfa84cac388b92fb04551d05e0ca3c11a66dd8ae))



# [1.1.0](https://github.com/rcourtman/Pulse/compare/v1.0.16...v1.1.0) (2025-03-03)


### Bug Fixes

* add 'unknown' status to ProxmoxContainer type ([e629152](https://github.com/rcourtman/Pulse/commit/e6291525593458db3f1f757bd84bb77beff251d2))
* enhance ProxmoxClient with configurable timeouts and improved error handling ([1ac5076](https://github.com/rcourtman/Pulse/commit/1ac5076e184476d40b83f0e21060cfa34dcf3692))
* implement moving average for network rates to reduce fluctuations ([7d91784](https://github.com/rcourtman/Pulse/commit/7d91784041dc02be041c84bf76cf626eabe21d35))
* multiply CPU values by 100 to convert from decimal to percentage ([370d278](https://github.com/rcourtman/Pulse/commit/370d278499bd355f018d7aa8da23bdd0a5f52533))
* reorder API routes to ensure they're defined before catch-all route ([b9897e5](https://github.com/rcourtman/Pulse/commit/b9897e531be18fb43acd3b2fa96fe910e0949e58))
* use environment variable for API timeout in all components ([5a30fcb](https://github.com/rcourtman/Pulse/commit/5a30fcb1c5b8ba3f7168a1335498ccd26ad113c4))


### Features

* add new API endpoint for retrieving all containers ([d7a859c](https://github.com/rcourtman/Pulse/commit/d7a859cefe55b247693b299b52c2f6a972cbfe15))



## [1.0.16](https://github.com/rcourtman/Pulse/compare/v1.0.15...v1.0.16) (2025-03-03)


### Bug Fixes

* add logs directory with proper permissions in Docker image ([39738c0](https://github.com/rcourtman/Pulse/commit/39738c0b4e0be443b4f48e1f83fa4d1688cbd022))


### Performance Improvements

* standardize polling intervals to fastest settings (1000ms) across all environments ([d68873c](https://github.com/rcourtman/Pulse/commit/d68873ce5de7580ce814cb0fc5283a73a32a470d))



## [1.0.15](https://github.com/rcourtman/Pulse/compare/v1.0.14...v1.0.15) (2025-03-03)



## [1.0.14](https://github.com/rcourtman/Pulse/compare/v1.0.13...v1.0.14) (2025-03-03)



## [1.0.13](https://github.com/rcourtman/Pulse/compare/v1.0.12...v1.0.13) (2025-03-02)



## [1.0.12](https://github.com/rcourtman/Pulse/compare/v1.0.11...v1.0.12) (2025-03-02)



## [1.0.11](https://github.com/rcourtman/Pulse/compare/v1.0.10...v1.0.11) (2025-03-02)



## [1.0.10](https://github.com/rcourtman/Pulse/compare/v1.0.9...v1.0.10) (2025-03-02)



## [1.0.9](https://github.com/rcourtman/Pulse/compare/v1.0.8...v1.0.9) (2025-03-02)



## [1.0.8](https://github.com/rcourtman/Pulse/compare/v1.0.7...v1.0.8) (2025-03-02)


### Reverts

* Revert "Fix: Improve filter badges layout and spacing" ([b08ff3c](https://github.com/rcourtman/Pulse/commit/b08ff3cce6a5d33f8b72bef69aa390569f037175))



## [1.0.7](https://github.com/rcourtman/Pulse/compare/v1.0.6...v1.0.7) (2025-03-02)



## [1.0.6](https://github.com/rcourtman/Pulse/compare/v1.0.5...v1.0.6) (2025-03-02)



## [1.0.5](https://github.com/rcourtman/Pulse/compare/v1.0.4...v1.0.5) (2025-03-02)



## [1.0.3](https://github.com/rcourtman/Pulse/compare/v1.0.2...v1.0.3) (2025-03-02)



## [1.0.2](https://github.com/rcourtman/Pulse/compare/v1.0.1...v1.0.2) (2025-03-02)



## 1.0.1 (2025-03-02)



