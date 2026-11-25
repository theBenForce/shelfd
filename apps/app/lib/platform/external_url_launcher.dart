import 'package:flutter/foundation.dart';
import 'package:url_launcher/url_launcher.dart';

typedef ExternalUrlLauncherCallback = Future<bool> Function(Uri uri);

ExternalUrlLauncherCallback? debugExternalUrlLauncherOverride;

Future<bool> launchExternalBrowserUrl(String rawUrl) async {
  final trimmed = rawUrl.trim();
  if (trimmed.isEmpty) return false;

  final uri = Uri.tryParse(trimmed);
  if (uri == null) return false;
  final scheme = uri.scheme.toLowerCase();
  if (scheme != 'http' && scheme != 'https') {
    return false;
  }

  final override = debugExternalUrlLauncherOverride;
  if (override != null) {
    return override(uri);
  }

  try {
    return await launchUrl(
      uri,
      mode: kIsWeb ? LaunchMode.platformDefault : LaunchMode.externalApplication,
    );
  } catch (_) {
    return false;
  }
}
