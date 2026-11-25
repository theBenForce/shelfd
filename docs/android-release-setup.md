# How to set up Google Play Android releases

Follow these steps once before you run automated Android releases from `.github/workflows/release.yml`. Google Play rejects automated API uploads until you create the application in Play Console and upload the first bundle manually.

## 1. Generate the Android upload keystore

Generate a dedicated upload keystore on a secure workstation. Do not commit the `.jks` file to Git.

```bash
keytool -genkeypair -v \
	-keystore shelfd-upload-keystore.jks \
	-storetype JKS \
	-keyalg RSA \
	-keysize 4096 \
	-validity 10000 \
	-alias shelfd-upload
```

Encode the keystore file as a single-line Base64 string for GitHub Actions:

```bash
base64 -w 0 shelfd-upload-keystore.jks > shelfd-upload-keystore.b64
```

To encode the keystore on macOS, run `base64 -i shelfd-upload-keystore.jks | tr -d '\n' > shelfd-upload-keystore.b64`.

## 2. Enroll in Play App Signing and upload versionCode 1 manually

Google Play requires one manual upload in the browser before the Play Developer API accepts builds for `com.justwriteapps.shelfd`.

1. Open Google Play Console and create a new app with package name `com.justwriteapps.shelfd`.
2. Open **Test and release > Setup > App signing** and enroll in Play App Signing.
3. Build a signed bootstrap bundle locally with `--build-number=1`:

```bash
cd apps/app
ANDROID_KEYSTORE_PATH=/abs/path/to/shelfd-upload-keystore.jks \
ANDROID_KEYSTORE_PASSWORD='<store-password>' \
ANDROID_KEY_ALIAS='shelfd-upload' \
ANDROID_KEY_PASSWORD='<key-password>' \
flutter build appbundle --release --build-name=1.0.0 --build-number=1
```

4. Open **Test and release > Testing > Internal testing**.
5. Create a new release, upload `apps/app/build/app/outputs/bundle/release/app-release.aab`, and roll out the release to internal testers.

## 3. Complete required Play Console declarations

Play Console blocks track rollouts on API 36 apps until you complete two policy forms under **Policy and programs > App content**:

1. **Foreground service permissions.** Open the `FOREGROUND_SERVICE_MEDIA_PLAYBACK` form and select audio playback for audiobooks and SMIL read-along narration. Record and link a short screen-recording walkthrough. In the video, start audio playback in Shelfd, lock or background the phone, and show the Android media notification controls continuing playback.
2. **App access.** Select that parts of the app require sign-in. Provide reviewer instructions with a reachable demo Shelfd URL, a test username, and a test password so Google reviewers can sign in on `ConnectView`.

Shelfd uses Google Play Services Code Scanner and declares zero `android.permission.CAMERA` permissions. Do not enable camera permission declarations.

## 4. Create the Google Play service account

1. Open the Google Cloud project linked to your Play Console developer account.
2. Enable the **Google Play Android Developer API**.
3. Open **IAM & Admin > Service Accounts** and create a service account named `shelfd-github-release`.
4. Create a JSON key for `shelfd-github-release` and download the JSON file.
5. Open Google Play Console, go to **Users and permissions**, click **Invite new users**, and enter the service account email address.
6. Open **App permissions**, add `com.justwriteapps.shelfd`, and grant these two permissions:
   * **View app information and download bulk reports.**
   * **Release to testing tracks.**
7. Click **Invite user** and accept the changes.

## 5. Configure GitHub Actions repository secrets

Open **Settings > Secrets and variables > Actions** in the GitHub repository and add these five repository secrets:

* `ANDROID_KEYSTORE_BASE64` set to the Base64 text from `shelfd-upload-keystore.b64`.
* `ANDROID_KEYSTORE_PASSWORD` set to the keystore password.
* `ANDROID_KEY_ALIAS` set to `shelfd-upload`.
* `ANDROID_KEY_PASSWORD` set to the key password.
* `GOOGLE_PLAY_SERVICE_ACCOUNT_JSON` set to the raw JSON contents of the Google Play service account key file.

Store the keystore and service account key in your password manager. Then delete `shelfd-upload-keystore.b64` and the downloaded JSON file from your workstation.

## 6. Run, retry, or roll back an internal release

### Standard release from `main`

1. Open **Actions > Release & Publish Container** in GitHub.
2. Click **Run workflow** on `main` with `dry_run` unchecked and `release_tag` empty.
3. Wait for `release` to cut `vX.Y.Z`. Then `build-image` and `publish-android` run in parallel.

### Retry or roll back with an existing tag

Use `release_tag` when a Google Play upload fails after `vX.Y.Z` is tagged, or when you want to roll the `internal` track back to an earlier tag without running `semantic-release` or overwriting `ghcr.io` `:latest`. Follow these steps:

1. Open **Actions > Release & Publish Container**.
2. Click **Run workflow** and enter the target Git tag in `release_tag`, such as `v1.31.0`.
3. Wait for `publish-android` to finish. Both `release` and `build-image` skip automatically. `publish-android` checks out `refs/tags/v1.31.0`, computes `100000 + github.run_number` as a higher `versionCode`, builds a signed `.aab`, and uploads the bundle to the Google Play `internal` track.

### Preserve monotonic `versionCode` values

`publish-android` computes `BUILD_NUMBER` as `100000 + github.run_number`. GitHub Actions scopes `github.run_number` to the exact file path `.github/workflows/release.yml`. If you rename or move `.github/workflows/release.yml`, `github.run_number` resets to `1`. Before you run a release from a renamed workflow file, raise the `100000` base offset above the highest `versionCode` already uploaded to Google Play.
