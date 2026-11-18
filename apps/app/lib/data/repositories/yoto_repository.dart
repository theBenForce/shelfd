import 'dart:convert';
import 'dart:math';
import 'dart:typed_data';
import '../models/yoto.dart';
import '../services/api_service.dart';
import '../services/storage_service.dart';

class YotoRepository {
  final ApiService apiService;
  final StorageService storageService;

  YotoRepository({
    required this.apiService,
    required this.storageService,
  });

  String? getSavedClientId() => storageService.getYotoClientId();

  Future<void> saveClientId(String clientId) =>
      storageService.saveYotoClientId(clientId.trim());

  Future<YotoStatus> getStatus() => apiService.getYotoStatus();

  /// Generates a PKCE code verifier, persists it locally, and returns the Yoto OAuth 2.0 authorization URL.
  Future<String> buildAuthorizationUrl({
    required String clientId,
    required String redirectUri,
    String authBaseUrl = 'https://login.yotoplay.com',
  }) async {
    final verifier = generatePkceVerifier();
    await storageService.saveYotoPkceVerifier(verifier);
    await storageService.saveYotoClientId(clientId.trim());

    final challenge = computePkceS256Challenge(verifier);
    final base = authBaseUrl.endsWith('/')
        ? authBaseUrl.substring(0, authBaseUrl.length - 1)
        : authBaseUrl;

    final uri = Uri.parse('$base/authorize').replace(
      queryParameters: {
        'audience': 'https://api.yotoplay.com',
        'scope': 'user:content:manage user:icons:manage offline_access',
        'response_type': 'code',
        'client_id': clientId.trim(),
        'code_challenge': challenge,
        'code_challenge_method': 'S256',
        'redirect_uri': redirectUri.trim(),
      },
    );
    return uri.toString();
  }

  Future<YotoStatus> exchangeOAuthCode({
    required String code,
    String? clientId,
    String? redirectUri,
    String? codeVerifier,
  }) async {
    final verifier = codeVerifier ?? storageService.getYotoPkceVerifier() ?? '';
    final effectiveClientId = clientId?.trim().isNotEmpty == true
        ? clientId!.trim()
        : storageService.getYotoClientId();

    final status = await apiService.exchangeYotoOAuth(
      code: code.trim(),
      codeVerifier: verifier,
      clientId: effectiveClientId,
      redirectUri: redirectUri?.trim(),
    );
    await storageService.clearYotoPkceVerifier();
    return status;
  }

  Future<YotoStatus> linkWithTokens({
    required String accessToken,
    String? refreshToken,
    String? clientId,
  }) async {
    if (clientId != null && clientId.trim().isNotEmpty) {
      await storageService.saveYotoClientId(clientId.trim());
    }
    return apiService.exchangeYotoOAuth(
      accessToken: accessToken.trim(),
      refreshToken: refreshToken?.trim(),
      clientId: clientId?.trim(),
    );
  }

  Future<void> unlinkAccount() async {
    await apiService.unlinkYotoAccount();
    await storageService.clearYotoPkceVerifier();
  }

  Future<List<YotoPlaylist>> getPlaylists() => apiService.getYotoPlaylists();

  Future<YotoIconsCatalog> getIcons() => apiService.getYotoIcons();

  Future<YotoDisplayIcon> uploadCustomIcon({
    required String filename,
    required List<int> bytes,
  }) =>
      apiService.uploadYotoIcon(filename: filename, bytes: bytes);

  Future<SendToYotoResult> sendBookToYoto(
    String bookId,
    SendToYotoRequest request,
  ) =>
      apiService.sendBookToYoto(bookId, request);

  Future<SendToYotoResult> sendSeriesToYoto(
    String seriesId,
    SendToYotoRequest request,
  ) =>
      apiService.sendSeriesToYoto(seriesId, request);

  static String generatePkceVerifier([int length = 64]) {
    const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~';
    final rand = Random.secure();
    return List.generate(length, (_) => chars[rand.nextInt(chars.length)]).join();
  }

  static String computePkceS256Challenge(String verifier) {
    final digest = _sha256(utf8.encode(verifier));
    return base64UrlEncode(digest).replaceAll('=', '');
  }

  static Uint8List _sha256(List<int> data) {
    const k = <int>[
      0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
      0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
      0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
      0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
      0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
      0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
      0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
      0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
    ];

    int rotr(int x, int n) => ((x & 0xffffffff) >>> n) | ((x << (32 - n)) & 0xffffffff);

    final bitLen = data.length * 8;
    final padded = <int>[...data, 0x80];
    while ((padded.length % 64) != 56) {
      padded.add(0);
    }
    final byteData = ByteData(8);
    final highBits = (bitLen ~/ 0x100000000) & 0xffffffff;
    final lowBits = bitLen & 0xffffffff;
    byteData.setUint32(0, highBits, Endian.big);
    byteData.setUint32(4, lowBits, Endian.big);
    for (int i = 0; i < 8; i++) {
      padded.add(byteData.getUint8(i));
    }

    int h0 = 0x6a09e667;
    int h1 = 0xbb67ae85;
    int h2 = 0x3c6ef372;
    int h3 = 0xa54ff53a;
    int h4 = 0x510e527f;
    int h5 = 0x9b05688c;
    int h6 = 0x1f83d9ab;
    int h7 = 0x5be0cd19;

    final w = List<int>.filled(64, 0);
    final chunkBytes = Uint8List.fromList(padded);
    final view = ByteData.sublistView(chunkBytes);

    for (int offset = 0; offset < chunkBytes.length; offset += 64) {
      for (int i = 0; i < 16; i++) {
        w[i] = view.getUint32(offset + i * 4, Endian.big);
      }
      for (int i = 16; i < 64; i++) {
        final s0 = rotr(w[i - 15], 7) ^ rotr(w[i - 15], 18) ^ ((w[i - 15] & 0xffffffff) >>> 3);
        final s1 = rotr(w[i - 2], 17) ^ rotr(w[i - 2], 19) ^ ((w[i - 2] & 0xffffffff) >>> 10);
        w[i] = (w[i - 16] + s0 + w[i - 7] + s1) & 0xffffffff;
      }

      int a = h0, b = h1, c = h2, d = h3, e = h4, f = h5, g = h6, h = h7;

      for (int i = 0; i < 64; i++) {
        final s1 = rotr(e, 6) ^ rotr(e, 11) ^ rotr(e, 25);
        final ch = (e & f) ^ ((~e) & g);
        final temp1 = (h + s1 + ch + k[i] + w[i]) & 0xffffffff;
        final s0 = rotr(a, 2) ^ rotr(a, 13) ^ rotr(a, 22);
        final maj = (a & b) ^ (a & c) ^ (b & c);
        final temp2 = (s0 + maj) & 0xffffffff;

        h = g;
        g = f;
        f = e;
        e = (d + temp1) & 0xffffffff;
        d = c;
        c = b;
        b = a;
        a = (temp1 + temp2) & 0xffffffff;
      }

      h0 = (h0 + a) & 0xffffffff;
      h1 = (h1 + b) & 0xffffffff;
      h2 = (h2 + c) & 0xffffffff;
      h3 = (h3 + d) & 0xffffffff;
      h4 = (h4 + e) & 0xffffffff;
      h5 = (h5 + f) & 0xffffffff;
      h6 = (h6 + g) & 0xffffffff;
      h7 = (h7 + h) & 0xffffffff;
    }

    final out = ByteData(32);
    out.setUint32(0, h0, Endian.big);
    out.setUint32(4, h1, Endian.big);
    out.setUint32(8, h2, Endian.big);
    out.setUint32(12, h3, Endian.big);
    out.setUint32(16, h4, Endian.big);
    out.setUint32(20, h5, Endian.big);
    out.setUint32(24, h6, Endian.big);
    out.setUint32(28, h7, Endian.big);
    return out.buffer.asUint8List();
  }
}
