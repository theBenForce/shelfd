import 'dart:convert';
import 'package:shared_preferences/shared_preferences.dart';

class StorageService {
  final SharedPreferences _prefs;

  StorageService(this._prefs);

  static const _keyServerUrl = 'shelfd_server_url';
  static const _keyAuthToken = 'shelfd_auth_token';
  static const _keySavedUsername = 'shelfd_saved_username';
  static const _keyThemeMode = 'shelfd_theme_mode';
  static const _keyFontSize = 'shelfd_font_size';
  static const _keyLineHeight = 'shelfd_line_height';
  static const _keyFontFamily = 'shelfd_font_family';
  static const _keyAutoCommitUploads = 'shelfd_auto_commit_uploads';
  static const _prefixProgress = 'shelfd_progress_';
  static const _prefixChapter = 'shelfd_chapter_';
  static const _prefixHighlights = 'shelfd_highlights_';

  // Server credentials
  String? getServerUrl() => _prefs.getString(_keyServerUrl);
  String? getAuthToken() => _prefs.getString(_keyAuthToken);
  String? getSavedUsername() => _prefs.getString(_keySavedUsername);

  Future<void> saveSavedUsername(String username) async {
    await _prefs.setString(_keySavedUsername, username);
  }

  Future<void> saveServerConnection(String url, String token, {String? username}) async {
    await _prefs.setString(_keyServerUrl, url);
    await _prefs.setString(_keyAuthToken, token);
    if (username != null && username.isNotEmpty) {
      await _prefs.setString(_keySavedUsername, username);
    }
  }

  Future<void> clearServerConnection() async {
    await _prefs.remove(_keyServerUrl);
    await _prefs.remove(_keyAuthToken);
  }

  // Reading settings
  String getThemeMode() => _prefs.getString(_keyThemeMode) ?? 'bone';
  Future<void> saveThemeMode(String mode) => _prefs.setString(_keyThemeMode, mode);

  double getFontSize() => _prefs.getDouble(_keyFontSize) ?? 18.0;
  Future<void> saveFontSize(double size) => _prefs.setDouble(_keyFontSize, size);

  double getLineHeight() => _prefs.getDouble(_keyLineHeight) ?? 1.6;
  Future<void> saveLineHeight(double height) => _prefs.setDouble(_keyLineHeight, height);

  String getFontFamily() => _prefs.getString(_keyFontFamily) ?? 'serif';
  Future<void> saveFontFamily(String family) => _prefs.setString(_keyFontFamily, family);

  // Upload preferences
  bool getAutoCommitUploads() => _prefs.getBool(_keyAutoCommitUploads) ?? false;
  Future<void> saveAutoCommitUploads(bool enabled) => _prefs.setBool(_keyAutoCommitUploads, enabled);

  // Reading progress
  double getReadingProgress(String bookId) {
    return _prefs.getDouble('$_prefixProgress$bookId') ?? 0.0;
  }

  Future<void> saveReadingProgress(String bookId, double progress) async {
    await _prefs.setDouble('$_prefixProgress$bookId', progress);
  }

  // Offline chapter caching
  Map<String, dynamic>? getCachedChapterData(String bookId, dynamic chapterIdentifier) {
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

  Future<void> cacheChapter(String bookId, dynamic chapterIdentifier, String content) async {
    await _prefs.setString('$_prefixChapter${bookId}_$chapterIdentifier', content);
  }

  Future<void> cacheChapterData(String bookId, dynamic chapterIdentifier, Map<String, dynamic> data) async {
    await _prefs.setString('$_prefixChapter${bookId}_$chapterIdentifier', jsonEncode(data));
  }

  // Offline highlights caching
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

  Future<void> cacheHighlights(String bookId, List<Map<String, dynamic>> highlights) async {
    await _prefs.setString('$_prefixHighlights$bookId', jsonEncode(highlights));
  }
}
