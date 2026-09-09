import 'dart:convert';
import 'package:shared_preferences/shared_preferences.dart';

class StorageService {
  final SharedPreferences _prefs;

  StorageService(this._prefs);

  static const _keyServerUrl = 'shelfd_server_url';
  static const _keyAuthToken = 'shelfd_auth_token';
  static const _keyThemeMode = 'shelfd_theme_mode';
  static const _keyFontSize = 'shelfd_font_size';
  static const _keyLineHeight = 'shelfd_line_height';
  static const _keyFontFamily = 'shelfd_font_family';
  static const _prefixProgress = 'shelfd_progress_';
  static const _prefixChapter = 'shelfd_chapter_';
  static const _prefixHighlights = 'shelfd_highlights_';

  // Server credentials
  String? getServerUrl() => _prefs.getString(_keyServerUrl);
  String? getAuthToken() => _prefs.getString(_keyAuthToken);

  Future<void> saveServerConnection(String url, String token) async {
    await _prefs.setString(_keyServerUrl, url);
    await _prefs.setString(_keyAuthToken, token);
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

  // Reading progress
  double getReadingProgress(String bookId) {
    return _prefs.getDouble('$_prefixProgress$bookId') ?? 0.0;
  }

  Future<void> saveReadingProgress(String bookId, double progress) async {
    await _prefs.setDouble('$_prefixProgress$bookId', progress);
  }

  // Offline chapter caching
  String? getCachedChapter(String bookId, dynamic chapterIdentifier) {
    return _prefs.getString('$_prefixChapter${bookId}_$chapterIdentifier');
  }

  Future<void> cacheChapter(String bookId, dynamic chapterIdentifier, String content) async {
    await _prefs.setString('$_prefixChapter${bookId}_$chapterIdentifier', content);
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
