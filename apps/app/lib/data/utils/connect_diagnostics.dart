import 'dart:async';
import '../services/api_service.dart';
import 'url_normalizer.dart';

class ConnectDiagnostic {
  final String message;
  final String clipboardDetails;

  const ConnectDiagnostic({
    required this.message,
    required this.clipboardDetails,
  });
}

class ConnectDiagnostics {
  static ConnectDiagnostic fromError(
    Object error, {
    String? serverUrl,
  }) {
    final safeUrl = UrlNormalizer.redactSensitiveTokens(serverUrl?.trim() ?? '');
    final rawError = UrlNormalizer.redactSensitiveTokens(error.toString());
    final lowerError = rawError.toLowerCase();
    final parsedHost = Uri.tryParse(safeUrl)?.host.toLowerCase() ?? '';
    final parsedScheme = Uri.tryParse(safeUrl)?.scheme.toLowerCase() ?? '';

    String friendly;

    if (error is FormatException) {
      friendly = error.message;
    } else if (error is ApiException) {
      friendly = _fromApiException(error);
    } else if (error is TimeoutException || lowerError.contains('timed out')) {
      if (UrlNormalizer.isTailscaleHost(parsedHost)) {
        friendly =
            'Connection timed out after 8 seconds. Check that Tailscale VPN is active on this device and your Shelfd server is online.';
      } else if (UrlNormalizer.isRfc1918OrCgnatIpv4(parsedHost) ||
          parsedHost.endsWith('.local') ||
          parsedHost.endsWith('.lan')) {
        friendly =
            'Connection timed out after 8 seconds. Check that this device is on the same Wi-Fi network as your server and Shelfd is bound to 0.0.0.0.';
      } else {
        friendly =
            'Server did not respond within 8 seconds. Verify the server URL, port, and firewall or reverse-proxy rules.';
      }
    } else if (lowerError.contains('handshakeexception') ||
        lowerError.contains('certificate_verify_failed') ||
        lowerError.contains('unable to get local issuer certificate') ||
        lowerError.contains('self signed certificate')) {
      friendly =
          'TLS certificate verification failed. If your homelab uses a private root CA, install the CA certificate in Android Settings (Encryption & credentials > Install a certificate > CA certificate), or check your reverse proxy HTTPS certificate.';
    } else if (lowerError.contains('wrong_version_number') ||
        lowerError.contains('tlsv1_alert') ||
        (parsedScheme == 'https' && lowerError.contains('record overflow'))) {
      friendly =
          'HTTPS handshake failed against a plain HTTP port. Try changing https:// to http:// (for example http://$parsedHost:8080).';
    } else if (lowerError.contains('cleartext http traffic') ||
        lowerError.contains('not permitted by network security policy')) {
      friendly =
          'Cleartext HTTP connection was blocked by OS network policy. Use https:// or verify your network configuration.';
    } else if (lowerError.contains('failed host lookup') ||
        lowerError.contains('no address associated with hostname') ||
        lowerError.contains('nodename nor servname provided')) {
      if (UrlNormalizer.isTailscaleHost(parsedHost)) {
        friendly =
            'Could not resolve Tailscale hostname "$parsedHost". Connect Tailscale VPN and enable MagicDNS, or use your server Tailscale 100.x IP.';
      } else {
        friendly =
            'Could not resolve hostname "$parsedHost". Check the spelling of your server domain or verify local DNS.';
      }
    } else if (lowerError.contains('connection refused')) {
      friendly =
          'Connection refused by $safeUrl. Verify that the Shelfd server container is running and the port number is correct.';
    } else if (lowerError.contains('network is unreachable') ||
        lowerError.contains('software caused connection abort') ||
        lowerError.contains('socketexception')) {
      if (UrlNormalizer.isTailscaleHost(parsedHost)) {
        friendly =
            'Cannot reach Tailscale network ($parsedHost). Turn on Tailscale VPN on this device and try again.';
      } else {
        friendly =
            'Cannot reach $safeUrl from your current network. Check Wi-Fi, VPN, or cellular connectivity.';
      }
    } else {
      friendly = rawError.isNotEmpty
          ? rawError
          : 'Unable to connect to Shelfd server.';
    }

    final summaryLines = <String>[
      'Shelfd Connection Diagnostics',
      if (safeUrl.isNotEmpty) 'Target URL: $safeUrl',
      'Summary: $friendly',
      'Raw Error: $rawError',
      'Timestamp: ${DateTime.now().toUtc().toIso8601String()}',
    ];

    return ConnectDiagnostic(
      message: friendly,
      clipboardDetails: summaryLines.join('\n'),
    );
  }

  static String _fromApiException(ApiException error) {
    final cleanMsg = UrlNormalizer.redactSensitiveTokens(error.message.trim());
    switch (error.statusCode) {
      case 401:
        if (cleanMsg.toLowerCase().contains('pairing')) {
          return 'Pairing QR code expired or was already used. Scan the refreshed QR code shown in Web Settings.';
        }
        return 'Invalid username or password.';
      case 429:
        return 'Too many pairing attempts. Wait one minute and scan a fresh QR code.';
      case 404:
      case 405:
        return 'Reached host, but this URL does not appear to be a current Shelfd server (/api/v1/server/connect-info returned ${error.statusCode}).';
      case 502:
      case 503:
      case 504:
        return 'Reverse proxy is reachable (HTTP ${error.statusCode}), but the Shelfd server behind it is offline or starting up.';
      default:
        if (cleanMsg.isNotEmpty) {
          return cleanMsg;
        }
        return 'Server returned HTTP ${error.statusCode}.';
    }
  }
}
