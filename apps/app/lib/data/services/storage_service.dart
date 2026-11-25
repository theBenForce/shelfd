import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:shared_preferences/shared_preferences.dart';

abstract class SecureTokenStorage {
  Future<String?> read(String key);
  Future<void> write(String key, String value);
  Future<void> delete(String key);
  Future<void> deleteAll();
}

class FlutterSecureTokenStorage implements SecureTokenStorage {
  final FlutterSecureStorage _storage;

  const FlutterSecureTokenStorage([
    this._storage = const FlutterSecureStorage(),
  ]);

  @override
  Future<String?> read(String key) => _storage.read(key: key);

  @override
  Future<void> write(String key, String value) =>
      _storage.write(key: key, value: value);

  @override
  Future<void> delete(String key) => _storage.delete(key: key);

  @override
  Future<void> deleteAll() => _storage.deleteAll();
}

class InMemorySecureTokenStorage implements SecureTokenStorage {
  final Map<String, String> _entries = <String, String>{};
  bool throwPlatformExceptionOnRead = false;

  Map<String, String> get entries => Map<String, String>.unmodifiable(_entries);

  @override
  Future<String?> read(String key) async {
    if (throwPlatformExceptionOnRead) {
      throw PlatformException(
        code: 'KEYSTORE_FAILURE',
        message: 'Simulated Android Keystore decryption error',
      );
    }
    return _entries[key];
  }

  @override
  Future<void> write(String key, String value) async {
    _entries[key] = value;
  }

  @override
  Future<void> delete(String key) async {
    _entries.remove(key);
  }

  @override
  Future<void> deleteAll() async {
    _entries.clear();
  }
}

class StorageService {
  final SharedPreferences _prefs;
  SecureTokenStorage? _secureStorage;
  bool _hydratedFromSecure = false;
  String? _cachedToken;
  String? _cachedTokenId;

  StorageService(this._prefs, {SecureTokenStorage? secureStorage})
      : _secureStorage = secureStorage;

  static const _keyServerUrl = 'shelfd_server_url';
  static const _keyAuthToken = 'shelfd_auth_token';
  static const _keyAuthTokenId = 'shelfd_auth_token_id';
  static const _keySavedUsername = 'shelfd_saved_username';
  static const _keyThemeMode = 'shelfd_theme_mode';
  static const _keyFontSize = 'shelfd_font_size';
  static const _keyLineHeight = 'shelfd_line_height';
  static const _keyFontFamily = 'shelfd_font_family';
  static const _keyAutoCommitUploads = 'shelfd_auto_commit_uploads';
  static const _prefixProgress = 'shelfd_progress_';
  static const _prefixAudioProgress = 'shelfd_audio_progress_';
  static const _prefixAudioSpeed = 'shelfd_audio_speed_';
  static const _prefixChapter = 'shelfd_chapter_';
  static const _prefixHighlights = 'shelfd_highlights_';
  static const _prefixPluginPkce = 'plugin_pkce_';
  static const _keyActivePkcePlugin = 'plugin_pkce_active_plugin';

  Future<void> init() async {
    if (kIsWeb && _secureStorage == null) {
      _cachedToken = _prefs.getString(_keyAuthToken);
      _cachedTokenId = _prefs.getString(_keyAuthTokenId);
      return;
    }

    _secureStorage ??= const FlutterSecureTokenStorage();
    final secure = _secureStorage!;

    try {
      var token = await secure.read(_keyAuthToken);
      var tokenId = await secure.read(_keyAuthTokenId);

      final legacyToken = _prefs.getString(_keyAuthToken);
      final legacyTokenId = _prefs.getString(_keyAuthTokenId);

      if ((token == null || token.isEmpty) &&
          legacyToken != null &&
          legacyToken.isNotEmpty) {
        await secure.write(_keyAuthToken, legacyToken);
        token = legacyToken;
      }
      if ((tokenId == null || tokenId.isEmpty) &&
          legacyTokenId != null &&
          legacyTokenId.isNotEmpty) {
        await secure.write(_keyAuthTokenId, legacyTokenId);
        tokenId = legacyTokenId;
      }

      if (legacyToken != null) {
        await _prefs.remove(_keyAuthToken);
      }
      if (legacyTokenId != null) {
        await _prefs.remove(_keyAuthTokenId);
      }

      _cachedToken = (token != null && token.isNotEmpty) ? token : null;
      _cachedTokenId = (tokenId != null && tokenId.isNotEmpty) ? tokenId : null;
      _hydratedFromSecure = true;
    } on PlatformException {
      try {
        await secure.deleteAll();
      } catch (_) {}
      await _prefs.remove(_keyAuthToken);
      await _prefs.remove(_keyAuthTokenId);
      _cachedToken = null;
      _cachedTokenId = null;
      _hydratedFromSecure = true;
    } on MissingPluginException {
      _cachedToken = _prefs.getString(_keyAuthToken);
      _cachedTokenId = _prefs.getString(_keyAuthTokenId);
      _secureStorage = null;
      _hydratedFromSecure = false;
    }
  }

  String? getServerUrl() => _prefs.getString(_keyServerUrl);

  String? getAuthToken() {
    if (_hydratedFromSecure) {
      return _cachedToken;
    }
    return _cachedToken ?? _prefs.getString(_keyAuthToken);
  }

  String? getAuthTokenId() {
    if (_hydratedFromSecure) {
      return _cachedTokenId;
    }
    return _cachedTokenId ?? _prefs.getString(_keyAuthTokenId);
  }

  String? getSavedUsername() => _prefs.getString(_keySavedUsername);

  Future<void> saveSavedUsername(String username) async {
    await _prefs.setString(_keySavedUsername, username);
  }

  Future<void> saveServerConnection(
    String url,
    String token, {
    String? username,
    String? tokenId,
  }) async {
    _cachedToken = token.isNotEmpty ? token : null;
    _cachedTokenId = (tokenId != null && tokenId.isNotEmpty) ? tokenId : null;

    await _prefs.setString(_keyServerUrl, url);
    if (username != null && username.isNotEmpty) {
      await _prefs.setString(_keySavedUsername, username);
    }

    final secure = _secureStorage;
    if (secure != null && (_hydratedFromSecure || !kIsWeb)) {
      _hydratedFromSecure = true;
      if (token.isNotEmpty) {
        await secure.write(_keyAuthToken, token);
      } else {
        await secure.delete(_keyAuthToken);
      }
      if (tokenId != null && tokenId.isNotEmpty) {
        await secure.write(_keyAuthTokenId, tokenId);
      } else {
        await secure.delete(_keyAuthTokenId);
      }
      await _prefs.remove(_keyAuthToken);
      await _prefs.remove(_keyAuthTokenId);
    } else {
      if (token.isNotEmpty) {
        await _prefs.setString(_keyAuthToken, token);
      } else {
        await _prefs.remove(_keyAuthToken);
      }
      if (tokenId != null && tokenId.isNotEmpty) {
        await _prefs.setString(_keyAuthTokenId, tokenId);
      } else {
        await _prefs.remove(_keyAuthTokenId);
      }
    }
  }

  Future<void> clearAuthToken() async {
    _cachedToken = null;
    _cachedTokenId = null;
    final secure = _secureStorage;
    if (secure != null) {
      try {
        await secure.delete(_keyAuthToken);
        await secure.delete(_keyAuthTokenId);
      } catch (_) {}
    }
    await _prefs.remove(_keyAuthToken);
    await _prefs.remove(_keyAuthTokenId);
  }

  Future<void> clearServerConnection() async {
    await _prefs.remove(_keyServerUrl);
    await clearAuthToken();
  }

  String getThemeMode() => _prefs.getString(_keyThemeMode) ?? 'bone';
  Future<void> saveThemeMode(String mode) =>
      _prefs.setString(_keyThemeMode, mode);

  double getFontSize() => _prefs.getDouble(_keyFontSize) ?? 18.0;
  Future<void> saveFontSize(double size) =>
      _prefs.setDouble(_keyFontSize, size);

  double getLineHeight() => _prefs.getDouble(_keyLineHeight) ?? 1.6;
  Future<void> saveLineHeight(double height) =>
      _prefs.setDouble(_keyLineHeight, height);

  String getFontFamily() => _prefs.getString(_keyFontFamily) ?? 'serif';
  Future<void> saveFontFamily(String family) =>
      _prefs.setString(_keyFontFamily, family);

  bool getAutoCommitUploads() => _prefs.getBool(_keyAutoCommitUploads) ?? false;
  Future<void> saveAutoCommitUploads(bool enabled) =>
      _prefs.setBool(_keyAutoCommitUploads, enabled);

  double getReadingProgress(String bookId) {
    return _prefs.getDouble('$_prefixProgress$bookId') ?? 0.0;
  }

  Future<void> saveReadingProgress(String bookId, double progress) async {
    await _prefs.setDouble('$_prefixProgress$bookId', progress);
  }

  double getAudiobookProgress(String bookId) {
    return _prefs.getDouble('$_prefixAudioProgress$bookId') ?? 0.0;
  }

  Future<void> saveAudiobookProgress(
    String bookId,
    double positionSeconds,
  ) async {
    await _prefs.setDouble('$_prefixAudioProgress$bookId', positionSeconds);
  }

  double getAudiobookSpeed(String bookId) {
    return _prefs.getDouble('$_prefixAudioSpeed$bookId') ?? 1.0;
  }

  Future<void> saveAudiobookSpeed(String bookId, double speed) async {
    await _prefs.setDouble('$_prefixAudioSpeed$bookId', speed);
  }

  Map<String, dynamic>? getCachedChapterData(
    String bookId,
    dynamic chapterIdentifier,
  ) {
    final raw = _prefs.getString('$_prefixChapter${bookId}_$chapterIdentifier');
    if (raw == null || raw.isEmpty) return null;
    try {
      final decoded = jsonDecode(raw);
      if (decoded is Map<String, dynamic>) {
        return decoded;
      }
    } catch (_) {}
    return null;
  }

  String? getCachedChapter(String bookId, dynamic chapterIdentifier) {
    final raw = _prefs.getString('$_prefixChapter${bookId}_$chapterIdentifier');
    if (raw == null || raw.isEmpty) return null;
    try {
      final decoded = jsonDecode(raw);
      if (decoded is Map<String, dynamic>) {
        return (decoded['content_plain'] ?? decoded['content']) as String?;
      }
    } catch (_) {}
    return raw;
  }

  Future<void> cacheChapter(
    String bookId,
    dynamic chapterIdentifier,
    String content,
  ) async {
    await _prefs.setString(
      '$_prefixChapter${bookId}_$chapterIdentifier',
      content,
    );
  }

  Future<void> cacheChapterData(
    String bookId,
    dynamic chapterIdentifier,
    Map<String, dynamic> data,
  ) async {
    await _prefs.setString(
      '$_prefixChapter${bookId}_$chapterIdentifier',
      jsonEncode(data),
    );
  }

  List<Map<String, dynamic>>? getCachedHighlights(String bookId) {
    final raw = _prefs.getString('$_prefixHighlights$bookId');
    if (raw == null || raw.isEmpty) return null;
    try {
      final list = jsonDecode(raw) as List<dynamic>;
      return list.whereType<Map<String, dynamic>>().toList();
    } catch (_) {
      return null;
    }
  }

  Future<void> cacheHighlights(
    String bookId,
    List<Map<String, dynamic>> highlights,
  ) async {
    await _prefs.setString('$_prefixHighlights$bookId', jsonEncode(highlights));
  }

  String? getPluginPkceVerifier(String pluginName) =>
      _prefs.getString('$_prefixPluginPkce$pluginName');

  Future<void> savePluginPkceVerifier(
    String pluginName,
    String verifier,
  ) async {
    await _prefs.setString('$_prefixPluginPkce$pluginName', verifier);
    await _prefs.setString(_keyActivePkcePlugin, pluginName);
  }

  Future<void> clearPluginPkceVerifier(String pluginName) async {
    await _prefs.remove('$_prefixPluginPkce$pluginName');
    if (_prefs.getString(_keyActivePkcePlugin) == pluginName) {
      await _prefs.remove(_keyActivePkcePlugin);
    }
  }

  String? getActivePkcePlugin() => _prefs.getString(_keyActivePkcePlugin);

  Future<void> saveActivePkcePlugin(String? pluginName) async {
    if (pluginName == null || pluginName.trim().isEmpty) {
      await _prefs.remove(_keyActivePkcePlugin);
    } else {
      await _prefs.setString(_keyActivePkcePlugin, pluginName.trim());
    }
  }
}
