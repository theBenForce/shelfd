import 'dart:convert';
import 'package:http/http.dart' as http;
import '../models/author.dart';
import '../models/book.dart';
import '../models/chapter.dart';
import '../models/connect_info.dart';
import '../models/genre.dart';
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

  Future<Chapter> getChapter(String bookId, int chapterIndex) async {
    final response = await client.get(
      _uri('/api/v1/books/$bookId/chapters/$chapterIndex'),
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
}
