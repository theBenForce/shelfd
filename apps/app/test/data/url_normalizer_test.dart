import 'dart:async';
import 'package:flutter_test/flutter_test.dart';
import 'package:shelf/data/services/api_service.dart';
import 'package:shelf/data/utils/connect_diagnostics.dart';
import 'package:shelf/data/utils/url_normalizer.dart';

void main() {
  group('UrlNormalizer', () {
    test('defaults bare public and Tailscale MagicDNS domains to https', () {
      final pub = UrlNormalizer.normalizeServerUrl('  shelfd.example.com/ ');
      expect(pub.url, 'https://shelfd.example.com');
      expect(pub.isCleartextHttp, isFalse);
      expect(pub.requiresCleartextWarning, isFalse);

      final ts = UrlNormalizer.normalizeServerUrl('shelfd.tailnet.ts.net');
      expect(ts.url, 'https://shelfd.tailnet.ts.net');
      expect(ts.isCleartextHttp, isFalse);
      expect(ts.isPrivateOrLocalNetwork, isTrue);
    });

    test('defaults RFC1918 LAN, Tailscale CGNAT, mDNS, and :8080 inputs to http', () {
      final lan = UrlNormalizer.normalizeServerUrl('192.168.1.50:8080/');
      expect(lan.url, 'http://192.168.1.50:8080');
      expect(lan.isCleartextHttp, isTrue);
      expect(lan.isPrivateOrLocalNetwork, isTrue);
      expect(lan.requiresCleartextWarning, isFalse);

      final cgnat = UrlNormalizer.normalizeServerUrl('100.101.102.103:8080');
      expect(cgnat.url, 'http://100.101.102.103:8080');
      expect(cgnat.isPrivateOrLocalNetwork, isTrue);
      expect(cgnat.requiresCleartextWarning, isFalse);

      final mdns = UrlNormalizer.normalizeServerUrl('homelab.local');
      expect(mdns.url, 'http://homelab.local');
      expect(mdns.requiresCleartextWarning, isFalse);
    });

    test('flags cleartext http on non-private public domains for confirmation', () {
      final pubHttp = UrlNormalizer.normalizeServerUrl('http://books.example.com');
      expect(pubHttp.url, 'http://books.example.com');
      expect(pubHttp.isCleartextHttp, isTrue);
      expect(pubHttp.isPrivateOrLocalNetwork, isFalse);
      expect(pubHttp.requiresCleartextWarning, isTrue);
    });

    test('strips accidental API and web route suffixes while keeping subpath', () {
      final withApi = UrlNormalizer.normalizeServerUrl(
        'https://homelab.example.com/shelfd/api/v1/',
      );
      expect(withApi.url, 'https://homelab.example.com/shelfd');

      final withConnect = UrlNormalizer.normalizeServerUrl(
        'https://shelfd.example.com/connect',
      );
      expect(withConnect.url, 'https://shelfd.example.com');
    });

    test('builds and parses shelfd://pair URIs and rejects localhost mobile targets', () {
      final uriStr = UrlNormalizer.buildPairingUri(
        serverUrl: 'https://shelfd.tailnet.ts.net/',
        code: 'abcdef0123456789',
      );
      expect(uriStr, startsWith('shelfd://pair?'));

      final parsed = UrlNormalizer.parsePairingInput(uriStr);
      expect(parsed.serverUrl, 'https://shelfd.tailnet.ts.net');
      expect(parsed.code, 'abcdef0123456789');

      final loopbackPair = UrlNormalizer.buildPairingUri(
        serverUrl: 'http://localhost:8080',
        code: 'abcdef0123456789',
      );
      expect(
        () => UrlNormalizer.parsePairingInput(loopbackPair),
        throwsA(isA<FormatException>()),
      );
    });

    test('isSameOrigin enforces exact scheme, host, and effective port match', () {
      expect(
        UrlNormalizer.isSameOrigin(
          'https://shelfd.example.com',
          'https://shelfd.example.com:443/api/v1/books/1/cover',
        ),
        isTrue,
      );
      expect(
        UrlNormalizer.isSameOrigin(
          'https://shelfd.example.com',
          '/api/v1/books/1/cover',
        ),
        isTrue,
      );
      expect(
        UrlNormalizer.isSameOrigin(
          'https://shelfd.example.com',
          'http://shelfd.example.com/api/v1/books/1/cover',
        ),
        isFalse,
      );
      expect(
        UrlNormalizer.isSameOrigin(
          'https://shelfd.example.com',
          'https://covers.openlibrary.org/b/id/123-L.jpg',
        ),
        isFalse,
      );
      expect(
        UrlNormalizer.isSameOrigin(
          'http://192.168.1.50:8080',
          'http://192.168.1.50:9000/api/v1/books/1/cover',
        ),
        isFalse,
      );
    });

    test('redacts query tokens, shelfd_ tokens, and Bearer headers', () {
      const raw =
          'GET https://host/api/v1/books/1/cover?token=shelfd_abc123def456 failed with Bearer eyJhbGciOiJIUzI1NiJ9.x.y';
      final redacted = UrlNormalizer.redactSensitiveTokens(raw);
      expect(redacted, isNot(contains('shelfd_abc123def456')));
      expect(redacted, isNot(contains('eyJhbGciOiJIUzI1NiJ9')));
      expect(redacted, contains('token=[REDACTED]'));
      expect(redacted, contains('Bearer [REDACTED]'));
    });

    test('sanitizes same-origin redirect paths and blocks open redirects', () {
      expect(
        UrlNormalizer.sanitizeSameOriginRedirect('/settings?plugin=yoto'),
        '/settings?plugin=yoto',
      );
      expect(
        UrlNormalizer.sanitizeSameOriginRedirect('https://evil.example/phish'),
        isNull,
      );
      expect(
        UrlNormalizer.sanitizeSameOriginRedirect('//evil.example/phish'),
        isNull,
      );
      expect(
        UrlNormalizer.sanitizeSameOriginRedirect('/connect?redirect=/books'),
        isNull,
      );
    });
  });

  group('ConnectDiagnostics', () {
    test('maps TLS certificate, Tailscale timeout, and HTTP errors with token redaction', () {
      final tls = ConnectDiagnostics.fromError(
        Exception('HandshakeException: CERTIFICATE_VERIFY_FAILED: self signed certificate'),
        serverUrl: 'https://shelfd.home.arpa',
      );
      expect(tls.message, contains('TLS certificate verification failed'));
      expect(tls.message, contains('Install a certificate'));

      final tsTimeout = ConnectDiagnostics.fromError(
        TimeoutException('after 8s'),
        serverUrl: 'https://shelfd.tailnet.ts.net',
      );
      expect(tsTimeout.message, contains('Tailscale VPN'));

      final pairExpired = ConnectDiagnostics.fromError(
        const ApiException(401, 'Invalid or expired pairing code ?token=shelfd_secret999'),
        serverUrl: 'http://192.168.1.50:8080',
      );
      expect(pairExpired.message, contains('Pairing QR code expired'));
      expect(pairExpired.clipboardDetails, isNot(contains('shelfd_secret999')));
    });
  });
}
