# Releasing

Tailclip uses GitHub Actions for both routine CI and tagged releases.

## Workflows

- `.github/workflows/ci.yml`: runs `go test ./...` on pushes to `main` and on pull requests, and verifies the Windows agent still builds.
- `.github/workflows/release.yml`: manual release pipeline triggered from GitHub Actions with a version like `v1.0.0`.

## What The Release Workflow Does

When you run the `Release` workflow from the GitHub Actions tab, it will:

1. Validate the version string.
2. Run `go test ./...`.
3. Build `tailclip-agent.exe` for `windows/amd64` using the Windows GUI subsystem.
4. Package a release zip that includes:
   - `tailclip-agent.exe`
   - `README.md`
   - `docs/config.example.json`
   - `docs/TASKER_SETUP.md`
   - `integrations/tasker/test-tailclip-endpoint.ps1`
5. Attach the importable Tasker profile `integrations/tasker/Tailclip.prf.xml`.
6. Generate release notes with:
   - a short quick-start section
   - a commit list since the previous tag
   - GitHub-generated release notes
7. Create the tag and publish the GitHub release.

## How To Publish A Release

1. Push your current branch to GitHub.
2. Run:

```powershell
.\scripts\release.ps1 -Version v1.0.0
```

3. Confirm the workflow finishes in GitHub Actions and inspect the published release page.

## Recommended Release Convention

- Use semantic version tags such as `v1.0.0`, `v1.0.1`, `v1.1.0`.
- Keep the release workflow manually triggered. It is safer than auto-publishing on every merge and still stays quick from terminal.
- Use the release page as the user-facing download surface:
  - the zip is the Windows install artifact
  - the Tasker profile is the Android import artifact
  - the release description is the short tutorial and changelog
