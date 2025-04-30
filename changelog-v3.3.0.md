# Changelog - v3.3.0

This release introduces responsive layout improvements, manual release triggering, and fixes issues with PBS stats and UI rendering.

## ✨ Features

*   **Responsive Layout:** Improved layout adjustments for the header, navigation tabs, and dashboard filters on different screen sizes. ([Commit](https://github.com/rcourtman/Pulse/commit/<<sha_for_responsive_layout>>)) <!-- Placeholder SHA -->
*   **Manual Workflow Trigger:** The 'Prepare Release' GitHub Actions workflow can now be triggered manually with specific inputs. ([Commit](https://github.com/rcourtman/Pulse/commit/<<sha_for_manual_trigger>>)) <!-- Placeholder SHA -->

## 🐛 Bug Fixes

*   **PBS Stats:** Correctly populate summary statistics in the `processPbsTasks` function. ([Commit](https://github.com/rcourtman/Pulse/commit/<<sha_for_pbs_fix>>)) <!-- Placeholder SHA -->
*   **UI:** Prevent a vertical scrollbar artifact from appearing unnecessarily on the tabs navigation bar. ([Commit](https://github.com/rcourtman/Pulse/commit/<<sha_for_scrollbar_fix>>)) <!-- Placeholder SHA --> 