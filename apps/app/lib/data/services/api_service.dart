import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import '../models/author.dart';
import '../models/background_job.dart';
import '../models/book.dart';
import '../models/book_chat.dart';
import '../models/bookmark.dart';
import '../models/cast_models.dart';
import '../models/chapter.dart';
import '../models/connect_info.dart';
import '../models/duplicate_group.dart';
import '../models/genre.dart';
import '../models/highlight.dart';
import '../models/queue_status.dart';
import '../models/search_result.dart';
import '../models/metadata_search_result.dart';
import '../models/series.dart';
import '../models/topic.dart';
import '../models/paginated_books.dart';
import '../models/plugin.dart';
import '../models/upload_job.dart';
import '../models/user.dart';
import '../utils/url_normalizer.dart';

class ApiException implements Exception {
  final int statusCode;
  final String message;

  const ApiException(this.statusCode, this.message);

  @override
  String toString() => 'ApiException($statusCode): $message';
}

class LoginResult {
  final String token;
  final String? tokenId;
  final User? user;

  const LoginResult({
    required this.token,
    this.tokenId,
    this.user,
  });
}

class PairingCodeSession {
  final String code;
  final DateTime expiresAt;

  const PairingCodeSession({
    required this.code,
    required this.expiresAt,
  });

  factory PairingCodeSession.fromJson(Map<String, dynamic> json) {
    final rawExpires = json['expires_at']?.toString() ?? '';
    return PairingCodeSession(
      code: json['code']?.toString() ?? '',
      expiresAt: DateTime.tryParse(rawExpires) ??
          DateTime.now().toUtc().add(const Duration(seconds: 90)),
    );
  }
}

class PairingExchangeResult {
  final String tokenId;
  final String name;
  final String token;
  final User user;

  const PairingExchangeResult({
    required this.tokenId,
    required this.name,
    required this.token,
    required this.user,
  });

  factory PairingExchangeResult.fromJson(Map<String, dynamic> json) {
    final rawUser = json['user'];
    final userMap = rawUser is Map<String, dynamic>
        ? rawUser
        : const <String, dynamic>{'id': 'current', 'username': 'user', 'role': 'user'};
    return PairingExchangeResult(
      tokenId: json['id']?.toString() ?? '',
      name: json['name']?.toString() ?? '',
      token: json['token']?.toString() ?? '',
      user: User.fromJson(userMap),
    );
  }
}

class ApiTokenInfo {
  final String id;
  final String name;
  final DateTime? createdAt;

  const ApiTokenInfo({
    required this.id,
    required this.name,
    this.createdAt,
  });

  factory ApiTokenInfo.fromJson(Map<String, dynamic> json) {
    final rawCreated = json['created_at']?.toString();
    return ApiTokenInfo(
      id: json['id']?.toString() ?? '',
      name: json['name']?.toString() ?? '',
      createdAt: rawCreated != null ? DateTime.tryParse(rawCreated) : null,
    );
  }
}

class BookMediaToken {
  final String token;
  final DateTime expiresAt;

  const BookMediaToken({
    required this.token,
    required this.expiresAt,
  });

  factory BookMediaToken.fromJson(Map<String, dynamic> json) {
    final rawExpires = json['expires_at']?.toString() ?? '';
    return BookMediaToken(
      token: json['token']?.toString() ?? '',
      expiresAt: DateTime.tryParse(rawExpires) ??
          DateTime.now().toUtc().add(const Duration(hours: 2)),
    );
  }
}

sealed class ShelfdEvent {
  const ShelfdEvent();
}

class QueueStatusEvent extends ShelfdEvent {
  final QueueStatus status;
  const QueueStatusEvent(this.status);
}

class BookAddedEvent extends ShelfdEvent {
  final Book book;
  const BookAddedEvent(this.book);
}

class BookUpdatedEvent extends ShelfdEvent {
  final Book book;
  const BookUpdatedEvent(this.book);
}

class ScanStatusEvent extends ShelfdEvent {
  final Map<String, dynamic> data;
  const ScanStatusEvent(this.data);
}

class PluginEvent extends ShelfdEvent {
  final Map<String, dynamic> data;
  const PluginEvent(this.data);
}

class JobUpdatedEvent extends ShelfdEvent {
  final BackgroundJob job;
  const JobUpdatedEvent(this.job);
}

class JobRemovedEvent extends ShelfdEvent {
  final String jobId;
  const JobRemovedEvent(this.jobId);
}

class CastStatusEvent extends ShelfdEvent {
  final CastSessionSnapshot session;
  const CastStatusEvent(this.session);
}

class CastDevicesEvent extends ShelfdEvent {
  final List<CastDevice> devices;
  const CastDevicesEvent(this.devices);
}

class ApiService {
  static const Duration defaultAuthTimeout = Duration(seconds: 8);

  String baseUrl;
  String? token;
  final http.Client client;
  final VoidCallback? onUnauthorized;
  final bool _isWeb;

  ApiService({
    required this.baseUrl,
    this.token,
    http.Client? client,
    this.onUnauthorized,
    bool? isWebOverride,
  })  : client = client ?? http.Client(),
        _isWeb = isWebOverride ?? kIsWeb;

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

  bool isSameOriginAsServer(String targetUrl) {
    return UrlNormalizer.isSameOrigin(baseUrl, targetUrl);
  }

  Map<String, String> _headers() {
    return {
      'Content-Type': 'application/json',
      'Accept': 'application/json',
      if (token != null && token!.isNotEmpty) 'Authorization': 'Bearer $token',
    };
  }

  String _extractErrorMessage(String rawBody, int statusCode) {
    if (rawBody.trim().isEmpty) {
      return 'HTTP $statusCode';
    }
    try {
      final decoded = jsonDecode(rawBody);
      if (decoded is Map<String, dynamic> && decoded['error'] != null) {
        return decoded['error'].toString();
      }
    } catch (_) {}
    return rawBody;
  }

  void _checkResponse(
    http.Response response, {
    bool notifyUnauthorized = true,
    Set<int> allowStatuses = const <int>{},
  }) {
    if (response.statusCode < 400 || allowStatuses.contains(response.statusCode)) {
      return;
    }
    if (notifyUnauthorized &&
        response.statusCode == 401 &&
        token != null &&
        token!.isNotEmpty) {
      onUnauthorized?.call();
    }
    throw ApiException(
      response.statusCode,
      _extractErrorMessage(response.body, response.statusCode),
    );
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

  Future<ServerConnectInfo> getConnectInfo({
    Duration timeout = defaultAuthTimeout,
  }) async {
    final response = await client
        .get(_uri('/api/v1/server/connect-info'), headers: _headers())
        .timeout(timeout);
    _checkResponse(response, notifyUnauthorized: false);
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return ServerConnectInfo.fromJson(data);
  }

  Future<LoginResult> loginWithDevice(
    String username,
    String password, {
    String? deviceName,
    Duration timeout = defaultAuthTimeout,
  }) async {
    final trimmedDevice = deviceName?.trim();
    final response = await client
        .post(
          _uri('/api/v1/auth/login'),
          headers: _headers(),
          body: jsonEncode({
            'username': username,
            'password': password,
            if (trimmedDevice != null && trimmedDevice.isNotEmpty)
              'device_name': trimmedDevice,
          }),
        )
        .timeout(timeout);
    _checkResponse(response, notifyUnauthorized: false);
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    final apiToken = data['api_token']?.toString();
    final jwtToken = data['token']?.toString();
    final resolvedToken =
        (apiToken != null && apiToken.isNotEmpty) ? apiToken : (jwtToken ?? '');
    final rawTokenId = data['api_token_id']?.toString();
    final rawUser = data['user'];
    final parsedUser =
        rawUser is Map<String, dynamic> ? User.fromJson(rawUser) : null;
    token = resolvedToken;
    return LoginResult(
      token: resolvedToken,
      tokenId: (rawTokenId != null && rawTokenId.isNotEmpty) ? rawTokenId : null,
      user: parsedUser,
    );
  }

  Future<String> login(
    String username,
    String password, {
    String? deviceName,
    Duration timeout = defaultAuthTimeout,
  }) async {
    final result = await loginWithDevice(
      username,
      password,
      deviceName: deviceName,
      timeout: timeout,
    );
    return result.token;
  }

  Future<PairingCodeSession> beginPairing({
    Duration timeout = defaultAuthTimeout,
  }) async {
    final response = await client
        .post(
          _uri('/api/v1/auth/pair/begin'),
          headers: _headers(),
          body: jsonEncode(const <String, dynamic>{}),
        )
        .timeout(timeout);
    _checkResponse(response);
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return PairingCodeSession.fromJson(data);
  }

  Future<PairingExchangeResult> exchangePairing(
    String code, {
    required String deviceName,
    Duration timeout = defaultAuthTimeout,
  }) async {
    final response = await client
        .post(
          _uri('/api/v1/auth/pair/exchange'),
          headers: _headers(),
          body: jsonEncode({
            'code': code.trim(),
            'device_name': deviceName.trim(),
          }),
        )
        .timeout(timeout);
    _checkResponse(response, notifyUnauthorized: false);
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    final result = PairingExchangeResult.fromJson(data);
    token = result.token;
    return result;
  }

  Future<({String id, String name, String token})> createApiToken(
    String name,
  ) async {
    final response = await client.post(
      _uri('/api/v1/auth/tokens'),
      headers: _headers(),
      body: jsonEncode({'name': name.trim()}),
    );
    _checkResponse(response);
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return (
      id: data['id']?.toString() ?? '',
      name: data['name']?.toString() ?? '',
      token: data['token']?.toString() ?? '',
    );
  }

  Future<List<ApiTokenInfo>> listApiTokens() async {
    final response = await client.get(
      _uri('/api/v1/auth/tokens'),
      headers: _headers(),
    );
    _checkResponse(response);
    final decoded = jsonDecode(response.body);
    final List<dynamic> items;
    if (decoded is Map<String, dynamic>) {
      items = decoded['tokens'] as List<dynamic>? ?? const [];
    } else if (decoded is List<dynamic>) {
      items = decoded;
    } else {
      items = const [];
    }
    return items
        .whereType<Map>()
        .map((e) => ApiTokenInfo.fromJson(Map<String, dynamic>.from(e)))
        .toList();
  }

  Future<void> deleteApiToken(
    String tokenId, {
    Duration timeout = const Duration(seconds: 3),
    bool notifyUnauthorized = true,
  }) async {
    final encoded = Uri.encodeComponent(tokenId);
    final response = await client
        .delete(
          _uri('/api/v1/auth/tokens/$encoded'),
          headers: _headers(),
        )
        .timeout(timeout);
    _checkResponse(
      response,
      notifyUnauthorized: notifyUnauthorized,
      allowStatuses: const <int>{404},
    );
  }

  Future<BookMediaToken> mintBookMediaToken(String bookId) async {
    final encoded = Uri.encodeComponent(bookId);
    final response = await client.post(
      _uri('/api/v1/books/$encoded/media-token'),
      headers: _headers(),
      body: jsonEncode(const <String, dynamic>{}),
    );
    _checkResponse(response);
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return BookMediaToken.fromJson(data);
  }

  Future<User> getMe() async {
    final response = await client.get(_uri('/api/v1/auth/me'), headers: _headers());
    _checkResponse(response);
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    final userPayload =
        data['user'] is Map<String, dynamic> ? data['user'] as Map<String, dynamic> : data;
    return User.fromJson(userPayload);
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
    _checkResponse(response, notifyUnauthorized: false);
  }

  Future<PaginatedBooks> getBooksPage({
    int page = 1,
    int perPage = 24,
    String? authorId,
    String? authorName,
    String? genreId,
    String? genreName,
    String? topicId,
    String? topicName,
    String? seriesId,
    String? search,
  }) async {
    final params = <String, dynamic>{
      'page': page,
      'per_page': perPage,
      'limit': perPage,
      'offset': (page - 1) * perPage,
      if (authorId != null && authorId.isNotEmpty) 'author_id': authorId,
      if (authorName != null && authorName.isNotEmpty) 'author': authorName,
      if (genreId != null && genreId.isNotEmpty) 'genre_id': genreId,
      if (genreName != null && genreName.isNotEmpty) 'genre': genreName,
      if (topicId != null && topicId.isNotEmpty) 'topic_id': topicId,
      if (topicName != null && topicName.isNotEmpty) 'topic': topicName,
      if (seriesId != null && seriesId.isNotEmpty) 'series_id': seriesId,
      if (search != null && search.isNotEmpty) 'search': search,
    };

    final response = await client.get(_uri('/api/v1/books', params), headers: _headers());
    _checkResponse(response);
    final body = jsonDecode(response.body);
    List<dynamic> data = [];
    int total = 0;
    int limit = perPage;
    int offset = (page - 1) * perPage;

    if (body is Map<String, dynamic>) {
      data = (body['books'] ?? body['data']) as List<dynamic>? ?? [];
      total = (body['total'] as num?)?.toInt() ?? data.length;
      limit = (body['limit'] as num?)?.toInt() ?? limit;
      offset = (body['offset'] as num?)?.toInt() ?? offset;
    } else if (body is List<dynamic>) {
      data = body;
      total = body.length;
    }
    final books = data
        .whereType<Map<String, dynamic>>()
        .map((b) => Book.fromJson(b, baseUrl: baseUrl))
        .toList();

    return PaginatedBooks(
      books: books,
      total: total,
      page: page,
      perPage: perPage,
      limit: limit,
      offset: offset,
    );
  }

  Future<List<Book>> getBooks({
    int page = 1,
    int perPage = 24,
    String? authorId,
    String? authorName,
    String? genreId,
    String? genreName,
    String? topicId,
    String? topicName,
    String? seriesId,
    String? search,
  }) async {
    final pageData = await getBooksPage(
      page: page,
      perPage: perPage,
      authorId: authorId,
      authorName: authorName,
      genreId: genreId,
      genreName: genreName,
      topicId: topicId,
      topicName: topicName,
      seriesId: seriesId,
      search: search,
    );
    return pageData.books;
  }

  Future<Book> getBook(String id) async {
    final response = await client.get(_uri('/api/v1/books/$id'), headers: _headers());
    _checkResponse(response);
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return Book.fromJson(data, baseUrl: baseUrl);
  }

  Future<ReadalongManifest> getBookReadalong(String id) async {
    final response = await client.get(_uri('/api/v1/books/$id/readalong'), headers: _headers());
    _checkResponse(response);
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return ReadalongManifest.fromJson(data);
  }

  String getAudioStreamUrl(String bookId) {
    if (_isWeb && token != null && token!.isNotEmpty) {
      return _uri('/api/v1/audiobooks/$bookId/stream', {'token': token!}).toString();
    }
    return _uri('/api/v1/audiobooks/$bookId/stream').toString();
  }

  String getBookFileStreamUrl(String bookId, String fileId) {
    if (_isWeb && token != null && token!.isNotEmpty) {
      return _uri('/api/v1/books/$bookId/files/$fileId/stream', {'token': token!}).toString();
    }
    return _uri('/api/v1/books/$bookId/files/$fileId/stream').toString();
  }

  Future<List<AudioChapter>> getAudioChapters(String bookId) async {
    final response = await client.get(_uri('/api/v1/audiobooks/$bookId/chapters'), headers: _headers());
    _checkResponse(response);
    final data = jsonDecode(response.body) as List<dynamic>? ?? [];
    return data.whereType<Map<String, dynamic>>().map((c) => AudioChapter.fromJson(c)).toList();
  }

  Future<AudiobookProgress> getAudiobookProgress(String bookId) async {
    final response = await client.get(_uri('/api/v1/audiobooks/$bookId/progress'), headers: _headers());
    _checkResponse(response);
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return AudiobookProgress.fromJson(data);
  }

  Future<AudiobookProgress> saveAudiobookProgress(
    String bookId, {
    required double positionSeconds,
    double speed = 1.0,
    bool isCompleted = false,
  }) async {
    final response = await client.post(
      _uri('/api/v1/audiobooks/$bookId/progress'),
      headers: _headers(),
      body: jsonEncode({
        'position_seconds': positionSeconds,
        'speed': speed,
        'is_completed': isCompleted,
      }),
    );
    _checkResponse(response);
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return AudiobookProgress.fromJson(data);
  }

  Future<void> deleteAudiobookProgress(String bookId) async {
    final response = await client.delete(_uri('/api/v1/audiobooks/$bookId/progress'), headers: _headers());
    _checkResponse(response, allowStatuses: const <int>{404});
  }

  String getBookDownloadUrl(String bookId) {
    if (_isWeb && token != null && token!.isNotEmpty) {
      return _uri('/api/v1/books/$bookId/download', {'token': token!}).toString();
    }
    return _uri('/api/v1/books/$bookId/download').toString();
  }

  Future<List<int>> downloadBookEpub(String bookId) async {
    final response = await client.get(
      _uri('/api/v1/books/$bookId/download'),
      headers: _headers(),
    );
    _checkResponse(response);
    return response.bodyBytes;
  }

  String getBookMarkdownUrl(String bookId) {
    if (_isWeb && token != null && token!.isNotEmpty) {
      return _uri('/api/v1/books/$bookId/markdown', {'token': token!}).toString();
    }
    return _uri('/api/v1/books/$bookId/markdown').toString();
  }

  Future<List<int>> downloadBookMarkdown(String bookId) async {
    final response = await client.get(
      _uri('/api/v1/books/$bookId/markdown'),
      headers: _headers(),
    );
    _checkResponse(response);
    return response.bodyBytes;
  }

  Future<List<int>> downloadStudyDeck(
    String bookId, {
    required String format,
    required List<int> chapterIndices,
  }) async {
    final encodedId = Uri.encodeComponent(bookId);
    final response = await client.post(
      _uri('/api/v1/books/$encodedId/study-deck'),
      headers: _headers(),
      body: jsonEncode({
        'format': format,
        if (chapterIndices.isNotEmpty) 'chapter_indices': chapterIndices,
      }),
    );
    _checkResponse(response);
    return response.bodyBytes;
  }

  Future<List<int>> downloadApiPathBytes(String apiPath) async {
    final normalizedPath = apiPath.startsWith('/') ? apiPath : '/$apiPath';
    final response = await client.get(
      _uri(normalizedPath),
      headers: _headers(),
    );
    _checkResponse(response);
    return response.bodyBytes;
  }

  Future<Chapter> getChapter(String bookId, dynamic chapterIdentifier) async {
    final response = await client.get(
      _uri('/api/v1/books/$bookId/chapters/$chapterIdentifier'),
      headers: _headers(),
    );
    _checkResponse(response);
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return Chapter.fromJson(data);
  }

  String getChapterHtmlUrl(String bookId, dynamic chapterIdentifier) {
    return _uri('/api/v1/books/$bookId/chapters/$chapterIdentifier/html').toString();
  }

  Future<String> getChapterHtml(String bookId, dynamic chapterIdentifier) async {
    final response = await client.get(
      _uri('/api/v1/books/$bookId/chapters/$chapterIdentifier/html'),
      headers: _headers(),
    );
    _checkResponse(response);
    return response.body;
  }

  Future<List<Author>> getAuthors() async {
    final response = await client.get(_uri('/api/v1/authors'), headers: _headers());
    _checkResponse(response);
    final body = jsonDecode(response.body);
    List<dynamic> data = [];
    if (body is Map<String, dynamic>) {
      data = (body['authors'] ?? body['data']) as List<dynamic>? ?? [];
    } else if (body is List<dynamic>) {
      data = body;
    }
    return data
        .whereType<Map<String, dynamic>>()
        .map((a) => Author.fromJson(a, baseUrl: baseUrl))
        .toList();
  }

  Future<List<Genre>> getGenres() async {
    final response = await client.get(_uri('/api/v1/genres'), headers: _headers());
    _checkResponse(response);
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

  Future<List<Topic>> getTopics() async {
    final response = await client.get(_uri('/api/v1/topics'), headers: _headers());
    _checkResponse(response);
    final body = jsonDecode(response.body);
    List<dynamic> data = [];
    if (body is Map<String, dynamic>) {
      data = (body['topics'] ?? body['data']) as List<dynamic>? ?? [];
    } else if (body is List<dynamic>) {
      data = body;
    }
    return data
        .whereType<Map<String, dynamic>>()
        .map((t) => Topic.fromJson(t))
        .toList();
  }

  Future<List<Series>> getSeries() async {
    final response = await client.get(_uri('/api/v1/series'), headers: _headers());
    _checkResponse(response);
    final body = jsonDecode(response.body);
    List<dynamic> data = [];
    if (body is Map<String, dynamic>) {
      data = (body['series'] ?? body['data']) as List<dynamic>? ?? [];
    } else if (body is List<dynamic>) {
      data = body;
    }
    return data
        .whereType<Map<String, dynamic>>()
        .map((s) => Series.fromJson(s, baseUrl: baseUrl))
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
    _checkResponse(response);
    final data = jsonDecode(response.body) as List<dynamic>? ?? [];
    return data.map((h) => SemanticSearchHit.fromJson(h as Map<String, dynamic>)).toList();
  }

  Future<BackgroundJob?> triggerLibraryScan() async {
    final response = await client.post(_uri('/api/v1/library/scan'), headers: _headers());
    _checkResponse(response);
    if (response.body.trim().isEmpty) return null;
    try {
      final decoded = jsonDecode(response.body);
      if (decoded is Map<String, dynamic> && decoded['job'] is Map) {
        return BackgroundJob.fromJson(
          Map<String, dynamic>.from(decoded['job'] as Map),
        );
      }
    } catch (_) {}
    return null;
  }

  Future<QueueStatus> getQueueStatus() async {
    final response = await client.get(_uri('/api/v1/queue/status'), headers: _headers());
    _checkResponse(response);
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return QueueStatus.fromJson(data);
  }

  Stream<ShelfdEvent> streamEvents() async* {
    final request = http.Request('GET', _uri('/api/v1/queue/events'));
    request.headers.addAll({
      'Accept': 'text/event-stream',
      'Cache-Control': 'no-cache',
      if (token != null && token!.isNotEmpty) 'Authorization': 'Bearer $token',
    });

    final response = await client.send(request);
    if (response.statusCode >= 400) {
      final body = await response.stream.bytesToString();
      if (response.statusCode == 401 && token != null && token!.isNotEmpty) {
        onUnauthorized?.call();
      }
      throw ApiException(
        response.statusCode,
        body.isNotEmpty
            ? _extractErrorMessage(body, response.statusCode)
            : 'Failed to connect to queue events stream',
      );
    }

    final lines = response.stream.transform(utf8.decoder).transform(const LineSplitter());
    String currentEvent = 'queue_status';

    await for (final line in lines) {
      final trimmed = line.trim();
      if (trimmed.isEmpty) {
        currentEvent = 'queue_status';
        continue;
      }

      if (trimmed.startsWith('event:')) {
        currentEvent = trimmed.substring(6).trim();
      } else if (trimmed.startsWith('data:')) {
        final dataStr = trimmed.substring(5).trim();
        if (dataStr.isNotEmpty) {
          try {
            final data = jsonDecode(dataStr) as Map<String, dynamic>;
            switch (currentEvent) {
              case 'queue_status':
                yield QueueStatusEvent(QueueStatus.fromJson(data));
                break;
              case 'book_added':
                yield BookAddedEvent(Book.fromJson(data, baseUrl: baseUrl));
                break;
              case 'book_updated':
                yield BookUpdatedEvent(Book.fromJson(data, baseUrl: baseUrl));
                break;
              case 'scan_status':
                yield ScanStatusEvent(data);
                break;
              case 'plugin_event':
                yield PluginEvent(data);
                break;
              case 'job_updated':
                yield JobUpdatedEvent(BackgroundJob.fromJson(data));
                break;
              case 'job_removed':
                final jobId = data['job_id']?.toString() ?? '';
                if (jobId.isNotEmpty) {
                  yield JobRemovedEvent(jobId);
                }
                break;
              case 'cast_status':
                yield CastStatusEvent(CastSessionSnapshot.fromJson(data));
                break;
              case 'cast_devices':
                final rawList = (data['devices'] as List?) ?? const [];
                final devices = rawList
                    .whereType<Map>()
                    .map((e) => CastDevice.fromJson(Map<String, dynamic>.from(e)))
                    .toList();
                yield CastDevicesEvent(devices);
                break;
            }
          } catch (_) {
            // Ignore malformed payloads
          }
        }
      }
    }
  }

  Stream<QueueStatus> streamQueueEvents() async* {
    await for (final event in streamEvents()) {
      if (event is QueueStatusEvent) {
        yield event.status;
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
    _checkResponse(response);
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return Bookmark.fromJson(data);
  }

  Future<List<Bookmark>> getBookmarks(String bookId) async {
    final response = await client.get(_uri('/api/v1/books/$bookId/bookmarks'), headers: _headers());
    _checkResponse(response);
    final data = jsonDecode(response.body) as List<dynamic>? ?? [];
    return data.whereType<Map<String, dynamic>>().map((b) => Bookmark.fromJson(b)).toList();
  }

  Future<void> deleteBookmark(String bookmarkId, {String? bookId}) async {
    final path = (bookId != null && bookId.isNotEmpty)
        ? '/api/v1/books/$bookId/bookmarks/$bookmarkId'
        : '/api/v1/bookmarks/$bookmarkId';
    final response = await client.delete(_uri(path), headers: _headers());
    _checkResponse(response);
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
    _checkResponse(response);
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return Highlight.fromJson(data);
  }

  Future<List<Highlight>> getHighlights(String bookId) async {
    final response = await client.get(_uri('/api/v1/books/$bookId/highlights'), headers: _headers());
    _checkResponse(response);
    final data = jsonDecode(response.body) as List<dynamic>? ?? [];
    return data.whereType<Map<String, dynamic>>().map((h) => Highlight.fromJson(h)).toList();
  }

  Future<void> deleteHighlight(String highlightId, {String? bookId}) async {
    final path = (bookId != null && bookId.isNotEmpty)
        ? '/api/v1/books/$bookId/highlights/$highlightId'
        : '/api/v1/highlights/$highlightId';
    final response = await client.delete(_uri(path), headers: _headers());
    _checkResponse(response);
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
    _checkResponse(response);
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return BookChatResponse.fromJson(data);
  }

  String getUploadJobCoverUrl(String jobId, {int? version}) {
    final cleanBase = baseUrl.endsWith('/') ? baseUrl.substring(0, baseUrl.length - 1) : baseUrl;
    final params = <String>[];
    if (version != null) {
      params.add('v=$version');
    }
    if (_isWeb && token != null && token!.isNotEmpty) {
      params.add('token=$token');
    }
    final queryString = params.isNotEmpty ? '?${params.join('&')}' : '';
    return '$cleanBase/api/v1/books/upload/jobs/$jobId/cover$queryString';
  }

  Future<void> uploadJobCover({
    required String jobId,
    required String filename,
    required List<int> bytes,
  }) async {
    final uri = _uri('/api/v1/books/upload/jobs/$jobId/cover');
    final request = http.MultipartRequest('POST', uri);
    if (token != null && token!.isNotEmpty) {
      request.headers['Authorization'] = 'Bearer $token';
    }
    request.files.add(
      http.MultipartFile.fromBytes(
        'cover',
        bytes,
        filename: filename,
      ),
    );

    final streamedResponse = await client.send(request);
    final response = await http.Response.fromStream(streamedResponse);
    _checkResponse(response);
  }

  Future<StagedUploadJob> stageUploadBook({
    required String filename,
    required List<int> bytes,
    String? coverFilename,
    List<int>? coverBytes,
  }) async {
    final uri = _uri('/api/v1/books/upload/stage');
    final request = http.MultipartRequest('POST', uri);
    if (token != null && token!.isNotEmpty) {
      request.headers['Authorization'] = 'Bearer $token';
    }
    request.files.add(
      http.MultipartFile.fromBytes(
        'file',
        bytes,
        filename: filename,
      ),
    );
    if (coverBytes != null && coverBytes.isNotEmpty) {
      request.files.add(
        http.MultipartFile.fromBytes(
          'cover',
          coverBytes,
          filename: coverFilename ?? 'cover.jpg',
        ),
      );
    }

    final streamedResponse = await client.send(request);
    final response = await http.Response.fromStream(streamedResponse);
    _checkResponse(response);
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return StagedUploadJob.fromJson(data);
  }

  Future<Book> commitUploadJob(String jobId, StagedMetadata metadata) async {
    final response = await client.post(
      _uri('/api/v1/books/upload/jobs/$jobId/commit'),
      headers: _headers(),
      body: jsonEncode(metadata.toJson()),
    );
    _checkResponse(response);
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return Book.fromJson(data, baseUrl: baseUrl);
  }

  Future<List<StagedUploadJob>> getStagedUploadJobs() async {
    final response = await client.get(
      _uri('/api/v1/books/upload/jobs?status=staged'),
      headers: _headers(),
    );
    _checkResponse(response);
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    final jobsList = data['jobs'] as List<dynamic>? ?? [];
    return jobsList
        .map((j) => StagedUploadJob.fromJson(j as Map<String, dynamic>))
        .toList();
  }

  Future<void> deleteUploadJob(String jobId) async {
    final response = await client.delete(
      _uri('/api/v1/books/upload/jobs/$jobId'),
      headers: _headers(),
    );
    _checkResponse(response, allowStatuses: const <int>{404});
  }

  Future<Book> updateBookMetadata(
    String bookId, {
    required String title,
    List<String>? authors,
    String? series,
    double? sequenceNumber,
    String? description,
    String? publisher,
    String? language,
    List<String>? genres,
    List<String>? topics,
  }) async {
    final response = await client.put(
      _uri('/api/v1/books/$bookId/metadata'),
      headers: _headers(),
      body: jsonEncode({
        'title': title,
        if (authors != null) 'authors': authors,
        if (series != null && series.isNotEmpty) 'series': series,
        if (sequenceNumber != null) 'sequence_number': sequenceNumber,
        if (description != null) 'description': description,
        if (publisher != null) 'publisher': publisher,
        if (language != null) 'language': language,
        if (genres != null) 'genres': genres,
        if (topics != null) 'topics': topics,
      }),
    );
    _checkResponse(response);
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return Book.fromJson(data, baseUrl: baseUrl);
  }

  Future<void> uploadBookCover({
    required String bookId,
    required String filename,
    required List<int> bytes,
  }) async {
    final uri = _uri('/api/v1/books/$bookId/cover');
    final request = http.MultipartRequest('POST', uri);
    if (token != null && token!.isNotEmpty) {
      request.headers['Authorization'] = 'Bearer $token';
    }
    request.files.add(
      http.MultipartFile.fromBytes(
        'cover',
        bytes,
        filename: filename,
      ),
    );

    final streamedResponse = await client.send(request);
    final response = await http.Response.fromStream(streamedResponse);
    _checkResponse(response);
  }

  Future<DuplicateScanResult> getDuplicateBooks() async {
    final response = await client.get(
      _uri('/api/v1/utilities/duplicates'),
      headers: _headers(),
    );
    _checkResponse(response);
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return DuplicateScanResult.fromJson(data);
  }

  Future<Book> mergeBooks({
    required String primaryBookId,
    required List<String> duplicateBookIds,
    MergeOptions? options,
  }) async {
    final response = await client.post(
      _uri('/api/v1/utilities/merge-books'),
      headers: _headers(),
      body: jsonEncode({
        'primary_book_id': primaryBookId,
        'duplicate_book_ids': duplicateBookIds,
        if (options != null) 'options': options.toJson(),
      }),
    );
    _checkResponse(response);
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    final bookData = data['book'] as Map<String, dynamic>? ?? data;
    return Book.fromJson(bookData, baseUrl: baseUrl);
  }

  Future<List<MetadataSearchResult>> searchMetadata({
    String? query,
    String? title,
    String? author,
    String? isbn,
    String? provider,
  }) async {
    final queryParams = <String, String>{};
    if (query != null && query.isNotEmpty) queryParams['q'] = query;
    if (title != null && title.isNotEmpty) queryParams['title'] = title;
    if (author != null && author.isNotEmpty) queryParams['author'] = author;
    if (isbn != null && isbn.isNotEmpty) queryParams['isbn'] = isbn;
    if (provider != null && provider.isNotEmpty) queryParams['provider'] = provider;

    final response = await client.get(
      _uri('/api/v1/metadata/search', queryParams.isNotEmpty ? queryParams : null),
      headers: _headers(),
    );
    _checkResponse(response);
    final list = jsonDecode(response.body) as List<dynamic>;
    return list
        .map((item) => MetadataSearchResult.fromJson(item as Map<String, dynamic>))
        .toList();
  }

  Future<void> fetchBookCoverFromUrl(String bookId, String coverUrl) async {
    final response = await client.post(
      _uri('/api/v1/books/$bookId/cover/fetch'),
      headers: _headers(),
      body: jsonEncode({'cover_url': coverUrl}),
    );
    _checkResponse(response);
  }

  Future<void> fetchJobCoverFromUrl(String jobId, String coverUrl) async {
    final response = await client.post(
      _uri('/api/v1/books/upload/jobs/$jobId/cover/fetch'),
      headers: _headers(),
      body: jsonEncode({'cover_url': coverUrl}),
    );
    _checkResponse(response);
  }

  // Plugin Management, Storefront Catalog, Auth & Declarative Actions Endpoints
  Never _throwPluginError(http.Response response) {
    _checkResponse(response);
    throw ApiException(
      response.statusCode,
      _extractErrorMessage(response.body, response.statusCode),
    );
  }

  Future<List<PluginSummary>> getPlugins() async {
    final response = await client.get(
      _uri('/api/v1/plugins'),
      headers: _headers(),
    );
    if (response.statusCode >= 400) {
      _throwPluginError(response);
    }
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    final list = data['plugins'] as List<dynamic>? ?? const [];
    return list
        .whereType<Map<String, dynamic>>()
        .map(PluginSummary.fromJson)
        .toList();
  }

  Future<List<PluginSummary>> discoverPlugins() async {
    final response = await client.post(
      _uri('/api/v1/plugins/discover'),
      headers: _headers(),
      body: jsonEncode(<String, dynamic>{}),
    );
    if (response.statusCode >= 400) {
      _throwPluginError(response);
    }
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    final list = data['plugins'] as List<dynamic>? ?? const [];
    return list
        .whereType<Map<String, dynamic>>()
        .map(PluginSummary.fromJson)
        .toList();
  }

  Future<PluginInstallResult> installPluginZip({
    required String filename,
    required List<int> bytes,
  }) async {
    return _uploadPluginZip(
      path: '/api/v1/plugins/install',
      filename: filename,
      bytes: bytes,
    );
  }

  Future<PluginInstallResult> updatePluginZip({
    required String name,
    required String filename,
    required List<int> bytes,
  }) async {
    final encoded = Uri.encodeComponent(name);
    return _uploadPluginZip(
      path: '/api/v1/plugins/$encoded/update',
      filename: filename,
      bytes: bytes,
    );
  }

  Future<PluginInstallResult> _uploadPluginZip({
    required String path,
    required String filename,
    required List<int> bytes,
  }) async {
    final uri = _uri(path);
    final request = http.MultipartRequest('POST', uri);
    if (token != null && token!.isNotEmpty) {
      request.headers['Authorization'] = 'Bearer $token';
    }
    request.files.add(
      http.MultipartFile.fromBytes(
        'file',
        bytes,
        filename: filename,
      ),
    );
    final streamedResponse = await client.send(request);
    final response = await http.Response.fromStream(streamedResponse);
    if (response.statusCode >= 400) {
      _throwPluginError(response);
    }
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return PluginInstallResult.fromJson(data);
  }

  Future<PluginDetail> getPluginDetail(String name) async {
    final encoded = Uri.encodeComponent(name);
    final response = await client.get(
      _uri('/api/v1/plugins/$encoded'),
      headers: _headers(),
    );
    if (response.statusCode >= 400) {
      _throwPluginError(response);
    }
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return PluginDetail.fromJson(data);
  }

  Future<PluginSummary> setPluginEnabled(String name, bool enabled) async {
    final encoded = Uri.encodeComponent(name);
    final action = enabled ? 'enable' : 'disable';
    final response = await client.post(
      _uri('/api/v1/plugins/$encoded/$action'),
      headers: _headers(),
      body: jsonEncode(<String, dynamic>{}),
    );
    if (response.statusCode >= 400) {
      _throwPluginError(response);
    }
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return PluginSummary.fromJson(data);
  }

  Future<PluginConfigResponse> getPluginConfig(String name) async {
    final encoded = Uri.encodeComponent(name);
    final response = await client.get(
      _uri('/api/v1/plugins/$encoded/config'),
      headers: _headers(),
    );
    if (response.statusCode >= 400) {
      _throwPluginError(response);
    }
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return PluginConfigResponse.fromJson(data);
  }

  Future<PluginConfigResponse> updatePluginConfig(
    String name,
    Map<String, dynamic> values,
  ) async {
    final encoded = Uri.encodeComponent(name);
    final response = await client.put(
      _uri('/api/v1/plugins/$encoded/config'),
      headers: _headers(),
      body: jsonEncode(values),
    );
    if (response.statusCode >= 400) {
      _throwPluginError(response);
    }
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return PluginConfigResponse.fromJson(data);
  }

  Future<Map<String, dynamic>> invokePluginCommand(
    String name,
    String command, [
    Map<String, dynamic>? args,
  ]) async {
    final encodedName = Uri.encodeComponent(name);
    final encodedCmd = Uri.encodeComponent(command);
    final response = await client.post(
      _uri('/api/v1/plugins/$encodedName/commands/$encodedCmd'),
      headers: _headers(),
      body: jsonEncode(args ?? const <String, dynamic>{}),
    );
    if (response.statusCode >= 400) {
      _throwPluginError(response);
    }
    final decoded = jsonDecode(response.body);
    if (decoded is Map<String, dynamic>) {
      return decoded;
    }
    return {'result': decoded};
  }

  Future<CatalogPage> getPluginCatalog(
    String name, {
    String? mode,
    String? query,
    String? group,
    String? cursor,
    String? filter,
    bool forceRefresh = false,
  }) async {
    final encoded = Uri.encodeComponent(name);
    final params = <String, dynamic>{
      if (mode != null && mode.isNotEmpty) 'mode': mode,
      if (query != null && query.isNotEmpty) 'q': query,
      if (group != null && group.isNotEmpty) 'group': group,
      if (cursor != null && cursor.isNotEmpty) 'cursor': cursor,
      if (filter != null && filter.isNotEmpty) 'filter': filter,
      if (forceRefresh) 'refresh': 'true',
    };
    final response = await client.get(
      _uri('/api/v1/plugins/$encoded/catalog', params.isNotEmpty ? params : null),
      headers: _headers(),
    );
    if (response.statusCode >= 400) {
      _throwPluginError(response);
    }
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return CatalogPage.fromJson(data);
  }

  Future<CatalogStageResult> stagePluginCatalogItems(
    String name,
    List<CatalogStageItemRequest> items, {
    bool async = false,
  }) async {
    final encoded = Uri.encodeComponent(name);
    final response = await client.post(
      _uri('/api/v1/plugins/$encoded/catalog/stage'),
      headers: _headers(),
      body: jsonEncode({
        'items': items.map((e) => e.toJson()).toList(),
        if (async) 'async': true,
      }),
    );
    if (response.statusCode >= 400) {
      _throwPluginError(response);
    }
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return CatalogStageResult.fromJson(data);
  }

  Future<PluginAuthStatus> getPluginAuthStatus(String pluginName) async {
    final encoded = Uri.encodeComponent(pluginName);
    final response = await client.get(
      _uri('/api/v1/plugins/$encoded/auth/status'),
      headers: _headers(),
    );
    if (response.statusCode >= 400) {
      _throwPluginError(response);
    }
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return PluginAuthStatus.fromJson(data);
  }

  Future<PluginAuthBeginResult> beginPluginAuth(
    String pluginName,
    Map<String, dynamic> body,
  ) async {
    final encoded = Uri.encodeComponent(pluginName);
    final response = await client.post(
      _uri('/api/v1/plugins/$encoded/auth/begin'),
      headers: _headers(),
      body: jsonEncode(body),
    );
    if (response.statusCode >= 400) {
      _throwPluginError(response);
    }
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return PluginAuthBeginResult.fromJson(data);
  }

  Future<PluginAuthStatus> completePluginAuth(
    String pluginName,
    Map<String, dynamic> body,
  ) async {
    final encoded = Uri.encodeComponent(pluginName);
    final response = await client.post(
      _uri('/api/v1/plugins/$encoded/auth/complete'),
      headers: _headers(),
      body: jsonEncode(body),
    );
    if (response.statusCode >= 400) {
      _throwPluginError(response);
    }
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return PluginAuthStatus.fromJson(data);
  }

  Future<PluginAuthStatus> disconnectPluginAuth(String pluginName) async {
    final encoded = Uri.encodeComponent(pluginName);
    final response = await client.post(
      _uri('/api/v1/plugins/$encoded/auth/disconnect'),
      headers: _headers(),
      body: jsonEncode(<String, dynamic>{}),
    );
    if (response.statusCode >= 400) {
      _throwPluginError(response);
    }
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return PluginAuthStatus.fromJson(data);
  }

  Future<PluginActionFormSpec> preparePluginAction(
    String pluginName,
    String actionName, {
    required PluginActionTarget target,
    Map<String, dynamic>? context,
  }) async {
    final encodedPlugin = Uri.encodeComponent(pluginName);
    final encodedAction = Uri.encodeComponent(actionName);
    final response = await client.post(
      _uri('/api/v1/plugins/$encodedPlugin/actions/$encodedAction/prepare'),
      headers: _headers(),
      body: jsonEncode({
        'target': target.toJson(),
        if (context != null && context.isNotEmpty) 'context': context,
      }),
    );
    if (response.statusCode >= 400) {
      _throwPluginError(response);
    }
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return PluginActionFormSpec.fromJson(data);
  }

  Future<PluginActionOptionsResult> getPluginActionOptions(
    String pluginName,
    String actionName, {
    required String source,
    String? tab,
    String? query,
    String? cursor,
    String? targetType,
    String? targetId,
  }) async {
    final encodedPlugin = Uri.encodeComponent(pluginName);
    final encodedAction = Uri.encodeComponent(actionName);
    final params = <String, dynamic>{
      'source': source,
      if (tab != null && tab.isNotEmpty) 'tab': tab,
      if (query != null && query.isNotEmpty) 'q': query,
      if (cursor != null && cursor.isNotEmpty) 'cursor': cursor,
      if (targetType != null && targetType.isNotEmpty) 'target_type': targetType,
      if (targetId != null && targetId.isNotEmpty) 'target_id': targetId,
    };
    final response = await client.get(
      _uri('/api/v1/plugins/$encodedPlugin/actions/$encodedAction/options', params),
      headers: _headers(),
    );
    if (response.statusCode >= 400) {
      _throwPluginError(response);
    }
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return PluginActionOptionsResult.fromJson(data);
  }

  Future<PluginOptionItem> uploadPluginActionAsset(
    String pluginName,
    String actionName, {
    required String filename,
    required List<int> bytes,
    String? field,
    String? mimeType,
  }) async {
    final encodedPlugin = Uri.encodeComponent(pluginName);
    final encodedAction = Uri.encodeComponent(actionName);
    final uri = _uri('/api/v1/plugins/$encodedPlugin/actions/$encodedAction/assets');
    final request = http.MultipartRequest('POST', uri);
    if (token != null && token!.isNotEmpty) {
      request.headers['Authorization'] = 'Bearer $token';
    }
    if (field != null && field.isNotEmpty) {
      request.fields['field'] = field;
    }
    request.files.add(
      http.MultipartFile.fromBytes(
        'file',
        bytes,
        filename: filename,
      ),
    );
    final streamedResponse = await client.send(request);
    final response = await http.Response.fromStream(streamedResponse);
    if (response.statusCode >= 400) {
      _throwPluginError(response);
    }
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    final itemJson = data['item'] is Map
        ? Map<String, dynamic>.from(data['item'] as Map)
        : data;
    return PluginOptionItem.fromJson(itemJson);
  }

  Future<PluginActionResult> invokePluginAction(
    String pluginName,
    String actionName, {
    required PluginActionTarget target,
    Map<String, dynamic>? input,
    Map<String, String>? overrides,
  }) async {
    final encodedPlugin = Uri.encodeComponent(pluginName);
    final encodedAction = Uri.encodeComponent(actionName);
    final response = await client.post(
      _uri('/api/v1/plugins/$encodedPlugin/actions/$encodedAction/invoke'),
      headers: _headers(),
      body: jsonEncode({
        'target': target.toJson(),
        if (input != null && input.isNotEmpty) 'values': input,
        if (overrides != null && overrides.isNotEmpty) 'overrides': overrides,
      }),
    );
    if (response.statusCode >= 400) {
      _throwPluginError(response);
    }
    final data = jsonDecode(response.body) as Map<String, dynamic>;
    return PluginActionResult.fromJson(data);
  }

  Future<List<PluginJob>> getPluginJobs() async {
    final response = await client.get(
      _uri('/api/v1/plugins/jobs'),
      headers: _headers(),
    );
    if (response.statusCode >= 400) {
      _throwPluginError(response);
    }
    final decoded = jsonDecode(response.body);
    final List<dynamic> list;
    if (decoded is Map<String, dynamic>) {
      list = decoded['jobs'] as List<dynamic>? ?? const [];
    } else if (decoded is List<dynamic>) {
      list = decoded;
    } else {
      list = const [];
    }
    return list
        .whereType<Map>()
        .map((e) => PluginJob.fromJson(Map<String, dynamic>.from(e)))
        .toList();
  }

  Future<void> dismissPluginJob(String jobId) async {
    final encoded = Uri.encodeComponent(jobId);
    final response = await client.delete(
      _uri('/api/v1/plugins/jobs/$encoded'),
      headers: _headers(),
    );
    if (response.statusCode >= 400 && response.statusCode != 404) {
      _throwPluginError(response);
    }
  }

  Future<List<BackgroundJob>> getBackgroundJobs() async {
    final response = await client.get(
      _uri('/api/v1/jobs'),
      headers: _headers(),
    );
    _checkResponse(response);
    final decoded = jsonDecode(response.body);
    final List<dynamic> list;
    if (decoded is Map<String, dynamic>) {
      list = decoded['jobs'] as List<dynamic>? ?? const [];
    } else if (decoded is List<dynamic>) {
      list = decoded;
    } else {
      list = const [];
    }
    return list
        .whereType<Map>()
        .map((e) => BackgroundJob.fromJson(Map<String, dynamic>.from(e)))
        .toList();
  }

  Future<void> dismissBackgroundJob(String jobId) async {
    final encoded = Uri.encodeComponent(jobId);
    final response = await client.delete(
      _uri('/api/v1/jobs/$encoded'),
      headers: _headers(),
    );
    _checkResponse(response, allowStatuses: const <int>{404});
  }

  Future<void> clearFinishedBackgroundJobs() async {
    final response = await client.delete(
      _uri('/api/v1/jobs'),
      headers: _headers(),
    );
    _checkResponse(response);
  }

  Future<List<CastDevice>> getCastDevices({bool refresh = false}) async {
    final path = refresh ? '/api/v1/cast/devices?refresh=true' : '/api/v1/cast/devices';
    final response = await client.get(_uri(path), headers: _headers());
    _checkResponse(response);
    final map = jsonDecode(response.body) as Map<String, dynamic>;
    final list = (map['devices'] as List?) ?? const [];
    return list
        .whereType<Map>()
        .map((e) => CastDevice.fromJson(Map<String, dynamic>.from(e)))
        .toList();
  }

  Future<CastSessionSnapshot?> getCastSession() async {
    final response = await client.get(_uri('/api/v1/cast/session'), headers: _headers());
    _checkResponse(response);
    final map = jsonDecode(response.body) as Map<String, dynamic>;
    final raw = map['session'];
    if (raw is! Map) return null;
    return CastSessionSnapshot.fromJson(Map<String, dynamic>.from(raw));
  }

  Future<CastSessionSnapshot> startCastSession({
    required String deviceId,
    required String bookId,
    String? fileId,
    double positionSeconds = 0.0,
    double speed = 1.0,
    bool captionsEnabled = false,
  }) async {
    final payload = <String, dynamic>{
      'device_id': deviceId,
      'book_id': bookId,
      if (fileId != null && fileId.isNotEmpty) 'file_id': fileId,
      'position_seconds': positionSeconds,
      'speed': speed,
      'captions_enabled': captionsEnabled,
    };
    final response = await client.post(
      _uri('/api/v1/cast/session'),
      headers: _headers(),
      body: jsonEncode(payload),
    );
    _checkResponse(response);
    final map = jsonDecode(response.body) as Map<String, dynamic>;
    return CastSessionSnapshot.fromJson(
      Map<String, dynamic>.from(map['session'] as Map),
    );
  }

  Future<CastSessionSnapshot?> controlCastSession({
    required String action,
    double? positionSeconds,
    double? speed,
    double? volume,
    bool? muted,
    bool? captionsEnabled,
  }) async {
    final payload = <String, dynamic>{
      'action': action,
      if (positionSeconds != null) 'position_seconds': positionSeconds,
      if (speed != null) 'speed': speed,
      if (volume != null) 'volume': volume,
      if (muted != null) 'muted': muted,
      if (captionsEnabled != null) 'captions_enabled': captionsEnabled,
    };
    final response = await client.post(
      _uri('/api/v1/cast/session/control'),
      headers: _headers(),
      body: jsonEncode(payload),
    );
    _checkResponse(response);
    final map = jsonDecode(response.body) as Map<String, dynamic>;
    final raw = map['session'];
    if (raw is! Map) return null;
    return CastSessionSnapshot.fromJson(Map<String, dynamic>.from(raw));
  }

  Future<void> stopCastSession() async {
    final response = await client.delete(
      _uri('/api/v1/cast/session'),
      headers: _headers(),
    );
    _checkResponse(response, allowStatuses: const <int>{404});
  }
}
