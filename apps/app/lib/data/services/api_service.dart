import 'dart:convert';
import 'package:http/http.dart' as http;
import '../models/author.dart';
import '../models/book.dart';
import '../models/book_chat.dart';
import '../models/bookmark.dart';
import '../models/chapter.dart';
import '../models/connect_info.dart';
import '../models/genre.dart';
import '../models/highlight.dart';
import '../models/queue_status.dart';
import '../models/search_result.dart';
import '../models/series.dart';
import '../models/user.dart';

class ApiException implements Exception {
  final int statusCode;
  final String message;

  const ApiException(this.statusCode, this.message);

  @override
  String toString() => 'ApiException($statusCode): $message';
}

class ApiService {
  String baseUrl;
  String? token;
  final http.Client client;

  ApiService({
    required this.baseUrl,
    this.token,
    http.Client? client,
  }) : client = client ?? http.Client();

  void updateConnection({String? newBaseUrl, String? newToken}) {
    if (newBaseUrl != null) {
      baseUrl = newBaseUrl.endsWith('/')
          ? newBaseUrl.substring(0, newBaseUrl.length - 1)
          : newBaseUrl;
    }
    if (newToken != null) {
      token = newToken;
    }
  }

  Map<String, String> _headers() {
    return {
      'Content-Type': 'application/json',
      'Accept': 'application/json',
      if (token != null && token!.isNotEmpty) 'Authorization': 'Bearer $token',
    };
  }

  Uri _uri(String path, [Map<String, dynamic>? queryParameters]) {
    final cleanBase = baseUrl.endsWith('/') ? baseUrl.substring(0, baseUrl.length - 1) : baseUrl;
    final fullUrl = '$cleanBase$path';
    final parsed = Uri.parse(fullUrl);
    if (queryParameters != null && queryParameters.isNotEmpty) {
      final stringParams = queryParameters.map((k, v) => MapEntry(k, v.toString()));
      return parsed.replace(queryParameters: stringParams);
    }
    return parsed;
  }

  Future<ServerConnectInfo> getConnectInfo() async {
    final response = await client.get(_uri('/api/v1/server/connect-info'), headers: _headers());
    if (response.statusCode >= 400) {
      throw ApiException(response.statusCode, response.body);
    }
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return ServerConnectInfo.fromJson(data);
  }

  Future<String> login(String username, String password) async {
    final response = await client.post(
      _uri('/api/v1/auth/login'),
      headers: _headers(),
      body: jsonEncode({'username': username, 'password': password}),
    );
    if (response.statusCode >= 400) {
      throw ApiException(response.statusCode, response.body);
    }
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    final token = data['token'] as String? ?? '';
    this.token = token;
    return token;
  }

  Future<User> getMe() async {
    final response = await client.get(_uri('/api/v1/auth/me'), headers: _headers());
    if (response.statusCode >= 400) {
      throw ApiException(response.statusCode, response.body);
    }
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return User.fromJson(data);
  }

  Future<void> changePassword(String currentPassword, String newPassword) async {
    final response = await client.post(
      _uri('/api/v1/auth/change-password'),
      headers: _headers(),
      body: jsonEncode({
        'current_password': currentPassword,
        'new_password': newPassword,
      }),
    );
    if (response.statusCode >= 400) {
      String errorMessage = response.body;
      try {
        final decoded = jsonDecode(response.body);
        if (decoded is Map<String, dynamic> && decoded.containsKey('error')) {
          errorMessage = decoded['error'].toString();
        }
      } catch (_) {}
      throw ApiException(response.statusCode, errorMessage);
    }
  }

  Future<List<Book>> getBooks({
    int page = 1,
    int perPage = 20,
    String? authorId,
    String? genreId,
    String? seriesId,
    String? search,
  }) async {
    final params = <String, dynamic>{
      'page': page,
      'per_page': perPage,
      if (authorId != null && authorId.isNotEmpty) 'author_id': authorId,
      if (genreId != null && genreId.isNotEmpty) 'genre_id': genreId,
      if (seriesId != null && seriesId.isNotEmpty) 'series_id': seriesId,
      if (search != null && search.isNotEmpty) 'search': search,
    };

    final response = await client.get(_uri('/api/v1/books', params), headers: _headers());
    if (response.statusCode >= 400) {
      throw ApiException(response.statusCode, response.body);
    }
    final body = jsonDecode(response.body);
    List<dynamic> data = [];
    if (body is Map<String, dynamic>) {
      data = (body['books'] ?? body['data']) as List<dynamic>? ?? [];
    } else if (body is List<dynamic>) {
      data = body;
    }
    return data
        .whereType<Map<String, dynamic>>()
        .map((b) => Book.fromJson(b, baseUrl: baseUrl))
        .toList();
  }

  Future<Book> getBook(String id) async {
    final response = await client.get(_uri('/api/v1/books/$id'), headers: _headers());
    if (response.statusCode >= 400) {
      throw ApiException(response.statusCode, response.body);
    }
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return Book.fromJson(data, baseUrl: baseUrl);
  }

  Future<Chapter> getChapter(String bookId, dynamic chapterIdentifier) async {
    final response = await client.get(
      _uri('/api/v1/books/$bookId/chapters/$chapterIdentifier'),
      headers: _headers(),
    );
    if (response.statusCode >= 400) {
      throw ApiException(response.statusCode, response.body);
    }
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return Chapter.fromJson(data);
  }

  Future<List<Author>> getAuthors() async {
    final response = await client.get(_uri('/api/v1/authors'), headers: _headers());
    if (response.statusCode >= 400) {
      throw ApiException(response.statusCode, response.body);
    }
    final body = jsonDecode(response.body);
    List<dynamic> data = [];
    if (body is Map<String, dynamic>) {
      data = (body['authors'] ?? body['data']) as List<dynamic>? ?? [];
    } else if (body is List<dynamic>) {
      data = body;
    }
    return data
        .whereType<Map<String, dynamic>>()
        .map((a) => Author.fromJson(a))
        .toList();
  }

  Future<List<Genre>> getGenres() async {
    final response = await client.get(_uri('/api/v1/genres'), headers: _headers());
    if (response.statusCode >= 400) {
      throw ApiException(response.statusCode, response.body);
    }
    final body = jsonDecode(response.body);
    List<dynamic> data = [];
    if (body is Map<String, dynamic>) {
      data = (body['genres'] ?? body['data']) as List<dynamic>? ?? [];
    } else if (body is List<dynamic>) {
      data = body;
    }
    return data
        .whereType<Map<String, dynamic>>()
        .map((g) => Genre.fromJson(g))
        .toList();
  }

  Future<List<Series>> getSeries() async {
    final response = await client.get(_uri('/api/v1/series'), headers: _headers());
    if (response.statusCode >= 400) {
      throw ApiException(response.statusCode, response.body);
    }
    final body = jsonDecode(response.body);
    List<dynamic> data = [];
    if (body is Map<String, dynamic>) {
      data = (body['series'] ?? body['data']) as List<dynamic>? ?? [];
    } else if (body is List<dynamic>) {
      data = body;
    }
    return data
        .whereType<Map<String, dynamic>>()
        .map((s) => Series.fromJson(s))
        .toList();
  }

  Future<List<SemanticSearchHit>> semanticSearch(
    String query, {
    int limit = 10,
    double minScore = 0.5,
  }) async {
    final params = <String, dynamic>{
      'q': query,
      'limit': limit,
      'min_score': minScore,
    };

    final response = await client.get(_uri('/api/v1/search', params), headers: _headers());
    if (response.statusCode >= 400) {
      throw ApiException(response.statusCode, response.body);
    }
    final data = jsonDecode(response.body) as List<dynamic>? ?? [];
    return data.map((h) => SemanticSearchHit.fromJson(h as Map<String, dynamic>)).toList();
  }

  Future<void> triggerLibraryScan() async {
    final response = await client.post(_uri('/api/v1/library/scan'), headers: _headers());
    if (response.statusCode >= 400) {
      throw ApiException(response.statusCode, response.body);
    }
  }

  Future<QueueStatus> getQueueStatus() async {
    final response = await client.get(_uri('/api/v1/queue/status'), headers: _headers());
    if (response.statusCode >= 400) {
      throw ApiException(response.statusCode, response.body);
    }
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return QueueStatus.fromJson(data);
  }

  Stream<QueueStatus> streamQueueEvents() async* {
    final request = http.Request('GET', _uri('/api/v1/queue/events'));
    request.headers.addAll({
      'Accept': 'text/event-stream',
      'Cache-Control': 'no-cache',
      if (token != null && token!.isNotEmpty) 'Authorization': 'Bearer $token',
    });

    final response = await client.send(request);
    if (response.statusCode >= 400) {
      final body = await response.stream.bytesToString();
      throw ApiException(
        response.statusCode,
        body.isNotEmpty ? body : 'Failed to connect to queue events stream',
      );
    }

    final lines = response.stream.transform(utf8.decoder).transform(const LineSplitter());

    await for (final line in lines) {
      if (line.startsWith('data: ')) {
        final dataStr = line.substring(6).trim();
        if (dataStr.isNotEmpty) {
          try {
            final data = jsonDecode(dataStr) as Map<String, dynamic>;
            yield QueueStatus.fromJson(data);
          } catch (_) {
            // Ignore malformed payloads
          }
        }
      }
    }
  }

  Future<Bookmark> createBookmark(
    String bookId, {
    required String title,
    double progress = 0.0,
    String? chapterId,
  }) async {
    final response = await client.post(
      _uri('/api/v1/books/$bookId/bookmarks'),
      headers: _headers(),
      body: jsonEncode({
        'title': title,
        'progress': progress,
        if (chapterId != null && chapterId.isNotEmpty) 'chapter_id': chapterId,
      }),
    );
    if (response.statusCode >= 400) {
      throw ApiException(response.statusCode, response.body);
    }
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return Bookmark.fromJson(data);
  }

  Future<List<Bookmark>> getBookmarks(String bookId) async {
    final response = await client.get(_uri('/api/v1/books/$bookId/bookmarks'), headers: _headers());
    if (response.statusCode >= 400) {
      throw ApiException(response.statusCode, response.body);
    }
    final data = jsonDecode(response.body) as List<dynamic>? ?? [];
    return data.whereType<Map<String, dynamic>>().map((b) => Bookmark.fromJson(b)).toList();
  }

  Future<void> deleteBookmark(String bookmarkId, {String? bookId}) async {
    final path = (bookId != null && bookId.isNotEmpty)
        ? '/api/v1/books/$bookId/bookmarks/$bookmarkId'
        : '/api/v1/bookmarks/$bookmarkId';
    final response = await client.delete(_uri(path), headers: _headers());
    if (response.statusCode >= 400) {
      throw ApiException(response.statusCode, response.body);
    }
  }

  Future<Highlight> createHighlight(
    String bookId, {
    required String selectedText,
    String color = 'yellow',
    String? note,
    String? chapterId,
    int? startOffset,
    int? endOffset,
    int? startParagraph,
    int? endParagraph,
    String? location,
  }) async {
    final response = await client.post(
      _uri('/api/v1/books/$bookId/highlights'),
      headers: _headers(),
      body: jsonEncode({
        'selected_text': selectedText,
        'color': color,
        if (note != null && note.isNotEmpty) 'note': note,
        if (chapterId != null && chapterId.isNotEmpty) 'chapter_id': chapterId,
        'start_offset': ?startOffset,
        'end_offset': ?endOffset,
        'start_paragraph': ?startParagraph,
        'end_paragraph': ?endParagraph,
        if (location != null && location.isNotEmpty) 'location': location,
      }),
    );
    if (response.statusCode >= 400) {
      throw ApiException(response.statusCode, response.body);
    }
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return Highlight.fromJson(data);
  }

  Future<List<Highlight>> getHighlights(String bookId) async {
    final response = await client.get(_uri('/api/v1/books/$bookId/highlights'), headers: _headers());
    if (response.statusCode >= 400) {
      throw ApiException(response.statusCode, response.body);
    }
    final data = jsonDecode(response.body) as List<dynamic>? ?? [];
    return data.whereType<Map<String, dynamic>>().map((h) => Highlight.fromJson(h)).toList();
  }

  Future<void> deleteHighlight(String highlightId, {String? bookId}) async {
    final path = (bookId != null && bookId.isNotEmpty)
        ? '/api/v1/books/$bookId/highlights/$highlightId'
        : '/api/v1/highlights/$highlightId';
    final response = await client.delete(_uri(path), headers: _headers());
    if (response.statusCode >= 400) {
      throw ApiException(response.statusCode, response.body);
    }
  }

  Future<BookChatResponse> chatWithBook(
    String bookId,
    String message, {
    List<Map<String, dynamic>>? history,
  }) async {
    final response = await client.post(
      _uri('/api/v1/books/$bookId/chat'),
      headers: _headers(),
      body: jsonEncode({
        'message': message,
        if (history != null && history.isNotEmpty) 'history': history,
      }),
    );
    if (response.statusCode >= 400) {
      throw ApiException(response.statusCode, response.body);
    }
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return BookChatResponse.fromJson(data);
  }
}
