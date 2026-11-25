class NormalizedServerUrl {
  final String url;
  final Uri uri;
  final bool isCleartextHttp;
  final bool isPrivateOrLocalNetwork;

  const NormalizedServerUrl({
    required this.url,
    required this.uri,
    required this.isCleartextHttp,
    required this.isPrivateOrLocalNetwork,
  });

  bool get requiresCleartextWarning =>
      isCleartextHttp && !isPrivateOrLocalNetwork;
}

class ParsedPairingUri {
  final String serverUrl;
  final String code;

  const ParsedPairingUri({
    required this.serverUrl,
    required this.code,
  });
}

class UrlNormalizer {
  static final RegExp _schemePrefix = RegExp(r'^[a-zA-Z][a-zA-Z0-9+\-.]*://');
  static final RegExp _ipv4Pattern =
      RegExp(r'^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$');
  static final RegExp _queryTokenPattern =
      RegExp(r'([?&]token=)[^&\s#]+', caseSensitive: false);
  static final RegExp _shelfdTokenPattern = RegExp(r'shelfd_[A-Za-z0-9_-]+');
  static final RegExp _bearerTokenPattern =
      RegExp(r'(Bearer\s+)[A-Za-z0-9._~+/-]+=*', caseSensitive: false);

  static NormalizedServerUrl normalizeServerUrl(String rawInput) {
    var trimmed = rawInput.trim();
    if (trimmed.isEmpty) {
      throw const FormatException('Enter your Shelfd server URL.');
    }

    if (_schemePrefix.hasMatch(trimmed)) {
      final lower = trimmed.toLowerCase();
      if (!lower.startsWith('http://') && !lower.startsWith('https://')) {
        throw const FormatException(
          'Server URL must start with https:// or http://.',
        );
      }
    } else {
      final inferredScheme = _inferDefaultScheme(trimmed);
      trimmed = '$inferredScheme://$trimmed';
    }

    final parsed = Uri.tryParse(trimmed);
    if (parsed == null || parsed.host.trim().isEmpty) {
      throw const FormatException('Enter a valid server hostname or IP address.');
    }

    if (parsed.hasPort && (parsed.port <= 0 || parsed.port > 65535)) {
      throw const FormatException('Server port must be between 1 and 65535.');
    }

    var cleanPath = parsed.path;
    while (cleanPath.endsWith('/')) {
      cleanPath = cleanPath.substring(0, cleanPath.length - 1);
    }

    const strippedSuffixes = <String>[
      '/api/v1/server/connect-info',
      '/api/v1',
      '/api',
      '/connect',
      '/books',
      '/settings',
    ];
    for (final suffix in strippedSuffixes) {
      if (cleanPath.toLowerCase().endsWith(suffix)) {
        cleanPath = cleanPath.substring(0, cleanPath.length - suffix.length);
        while (cleanPath.endsWith('/')) {
          cleanPath = cleanPath.substring(0, cleanPath.length - 1);
        }
        break;
      }
    }

    final normalizedUri = Uri(
      scheme: parsed.scheme.toLowerCase(),
      host: parsed.host.toLowerCase(),
      port: parsed.hasPort ? parsed.port : null,
      path: cleanPath.isEmpty ? '' : cleanPath,
    );

    var serialized = normalizedUri.toString();
    if (serialized.endsWith('/')) {
      serialized = serialized.substring(0, serialized.length - 1);
    }

    final isHttp = normalizedUri.scheme == 'http';
    final isPrivate = isPrivateOrTrustedLocalHost(normalizedUri.host);

    return NormalizedServerUrl(
      url: serialized,
      uri: normalizedUri,
      isCleartextHttp: isHttp,
      isPrivateOrLocalNetwork: isPrivate,
    );
  }

  static String _inferDefaultScheme(String bareAuthorityAndPath) {
    final slashIdx = bareAuthorityAndPath.indexOf('/');
    final authority = slashIdx >= 0
        ? bareAuthorityAndPath.substring(0, slashIdx)
        : bareAuthorityAndPath;

    final probe = Uri.tryParse('http://$authority');
    if (probe == null || probe.host.isEmpty) {
      return 'https';
    }

    final host = probe.host.toLowerCase();
    if (probe.hasPort) {
      if (probe.port == 443 || probe.port == 8443) {
        return 'https';
      }
      if (probe.port == 8080 || probe.port == 80 || probe.port == 3000) {
        return 'http';
      }
    }

    if (isLoopbackHost(host) ||
        isRfc1918OrCgnatIpv4(host) ||
        host.endsWith('.local') ||
        host.endsWith('.lan') ||
        host.endsWith('.home.arpa') ||
        !host.contains('.')) {
      return 'http';
    }

    return 'https';
  }

  static bool isLoopbackHost(String rawHost) {
    final host = _stripIpv6Brackets(rawHost.trim().toLowerCase());
    if (host == 'localhost' || host == '::1' || host == '0.0.0.0') {
      return true;
    }
    final match = _ipv4Pattern.firstMatch(host);
    if (match != null) {
      final a = int.tryParse(match.group(1)!) ?? -1;
      return a == 127;
    }
    return false;
  }

  static bool isRfc1918OrCgnatIpv4(String rawHost) {
    final host = _stripIpv6Brackets(rawHost.trim().toLowerCase());
    final match = _ipv4Pattern.firstMatch(host);
    if (match == null) return false;
    final a = int.tryParse(match.group(1)!) ?? -1;
    final b = int.tryParse(match.group(2)!) ?? -1;
    final c = int.tryParse(match.group(3)!) ?? -1;
    final d = int.tryParse(match.group(4)!) ?? -1;
    if (a < 0 || a > 255 || b < 0 || b > 255 || c < 0 || c > 255 || d < 0 || d > 255) {
      return false;
    }
    if (a == 10) return true;
    if (a == 192 && b == 168) return true;
    if (a == 172 && b >= 16 && b <= 31) return true;
    if (a == 100 && b >= 64 && b <= 127) return true;
    return false;
  }

  static bool isTailscaleHost(String rawHost) {
    final host = _stripIpv6Brackets(rawHost.trim().toLowerCase());
    if (host.endsWith('.ts.net')) return true;
    final match = _ipv4Pattern.firstMatch(host);
    if (match == null) return false;
    final a = int.tryParse(match.group(1)!) ?? -1;
    final b = int.tryParse(match.group(2)!) ?? -1;
    return a == 100 && b >= 64 && b <= 127;
  }

  static bool isPrivateOrTrustedLocalHost(String rawHost) {
    final host = _stripIpv6Brackets(rawHost.trim().toLowerCase());
    if (host.isEmpty) return false;
    if (isLoopbackHost(host) || isRfc1918OrCgnatIpv4(host)) {
      return true;
    }
    if (host.startsWith('fd') || host.startsWith('fe80:')) {
      return true;
    }
    if (host.endsWith('.local') ||
        host.endsWith('.lan') ||
        host.endsWith('.home.arpa') ||
        host.endsWith('.ts.net') ||
        !host.contains('.')) {
      return true;
    }
    return false;
  }

  static String buildPairingUri({
    required String serverUrl,
    required String code,
  }) {
    final normalized = normalizeServerUrl(serverUrl);
    final trimmedCode = code.trim();
    if (trimmedCode.isEmpty) {
      throw const FormatException('Pairing code is required.');
    }
    return Uri(
      scheme: 'shelfd',
      host: 'pair',
      queryParameters: <String, String>{
        'u': normalized.url,
        'c': trimmedCode,
      },
    ).toString();
  }

  static ParsedPairingUri parsePairingInput(
    String rawInput, {
    bool rejectLoopback = true,
  }) {
    final trimmed = rawInput.trim();
    if (trimmed.isEmpty) {
      throw const FormatException('Pairing link is empty.');
    }

    final uri = Uri.tryParse(trimmed);
    if (uri == null) {
      throw const FormatException('Invalid Shelfd pairing QR code.');
    }

    final rawUrl = uri.queryParameters['u'] ?? uri.queryParameters['url'];
    final rawCode = uri.queryParameters['c'] ?? uri.queryParameters['code'];

    if (rawUrl == null ||
        rawUrl.trim().isEmpty ||
        rawCode == null ||
        rawCode.trim().isEmpty) {
      throw const FormatException(
        'QR code is not a valid Shelfd pairing link.',
      );
    }

    final normalized = normalizeServerUrl(rawUrl);
    if (rejectLoopback && isLoopbackHost(normalized.uri.host)) {
      throw const FormatException(
        'This QR code uses localhost, which points to this phone instead of your server. '
        'Enter your server LAN IP or Tailscale address in the web pairing dialog and scan again.',
      );
    }

    return ParsedPairingUri(
      serverUrl: normalized.url,
      code: rawCode.trim(),
    );
  }

  static bool isSameOrigin(String serverBaseUrl, String targetUrl) {
    final trimmedTarget = targetUrl.trim();
    if (trimmedTarget.isEmpty) return false;
    if (trimmedTarget.startsWith('/') && !trimmedTarget.startsWith('//')) {
      return true;
    }

    final trimmedServer = serverBaseUrl.trim();
    if (trimmedServer.isEmpty) return false;

    final serverUri = Uri.tryParse(trimmedServer);
    final targetUri = Uri.tryParse(trimmedTarget);
    if (serverUri == null ||
        targetUri == null ||
        !serverUri.hasScheme ||
        !targetUri.hasScheme ||
        serverUri.host.isEmpty ||
        targetUri.host.isEmpty) {
      return false;
    }

    final serverScheme = serverUri.scheme.toLowerCase();
    final targetScheme = targetUri.scheme.toLowerCase();
    if (serverScheme != targetScheme) {
      return false;
    }

    if (serverUri.host.toLowerCase() != targetUri.host.toLowerCase()) {
      return false;
    }

    return _effectivePort(serverUri) == _effectivePort(targetUri);
  }

  static String? sanitizeSameOriginRedirect(String? rawRedirect) {
    if (rawRedirect == null) return null;
    final trimmed = rawRedirect.trim();
    if (trimmed.isEmpty) return null;
    if (!trimmed.startsWith('/') || trimmed.startsWith('//')) {
      return null;
    }
    if (trimmed.startsWith('/connect')) {
      return null;
    }
    return trimmed;
  }

  static String redactSensitiveTokens(String input) {
    if (input.isEmpty) return input;
    var redacted = input.replaceAllMapped(
      _queryTokenPattern,
      (m) => '${m.group(1)}[REDACTED]',
    );
    redacted = redacted.replaceAll(_shelfdTokenPattern, 'shelfd_[REDACTED]');
    redacted = redacted.replaceAllMapped(
      _bearerTokenPattern,
      (m) => '${m.group(1)}[REDACTED]',
    );
    return redacted;
  }

  static int _effectivePort(Uri uri) {
    if (uri.hasPort && uri.port > 0) return uri.port;
    switch (uri.scheme.toLowerCase()) {
      case 'https':
        return 443;
      case 'http':
        return 80;
      default:
        return 0;
    }
  }

  static String _stripIpv6Brackets(String host) {
    if (host.startsWith('[') && host.endsWith(']') && host.length > 2) {
      return host.substring(1, host.length - 1);
    }
    return host;
  }
}
