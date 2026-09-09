import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';
import '../../data/models/author.dart';
import '../../data/models/book.dart';
import '../../data/models/book_chat.dart';
import '../../data/models/genre.dart';
import '../../data/models/queue_status.dart';
import '../../data/models/search_result.dart';
import '../../data/models/series.dart';
import '../../data/models/user.dart';
import '../../data/repositories/auth_repository.dart';
import '../../data/repositories/book_repository.dart';
import '../../data/repositories/reader_repository.dart';
import '../../data/services/api_service.dart';
import '../../data/services/storage_service.dart';
import '../core/theme.dart';

// Service & Repository Providers
final sharedPreferencesProvider = Provider<SharedPreferences>((ref) {
  throw UnimplementedError('Must override sharedPreferencesProvider');
});

final storageServiceProvider = Provider<StorageService>((ref) {
  final prefs = ref.watch(sharedPreferencesProvider);
  return StorageService(prefs);
});

final apiServiceProvider = Provider<ApiService>((ref) {
  final storage = ref.watch(storageServiceProvider);
  final serverUrl = kIsWeb ? Uri.base.origin : (storage.getServerUrl() ?? 'http://localhost:8080');
  final token = storage.getAuthToken();
  return ApiService(baseUrl: serverUrl, token: token);
});

final authRepositoryProvider = Provider<AuthRepository>((ref) {
  return AuthRepository(
    apiService: ref.watch(apiServiceProvider),
    storageService: ref.watch(storageServiceProvider),
  );
});

final bookRepositoryProvider = Provider<BookRepository>((ref) {
  return BookRepository(
    apiService: ref.watch(apiServiceProvider),
    storageService: ref.watch(storageServiceProvider),
  );
});

final readerRepositoryProvider = Provider<ReaderRepository>((ref) {
  return ReaderRepository(
    apiService: ref.watch(apiServiceProvider),
    storageService: ref.watch(storageServiceProvider),
  );
});

// Auth State
class AuthState {
  final User? user;
  final bool isLoading;
  final String? error;
  final String? serverUrl;

  const AuthState({
    this.user,
    this.isLoading = false,
    this.error,
    this.serverUrl,
  });

  bool get isAuthenticated => user != null;

  AuthState copyWith({
    User? user,
    bool? isLoading,
    String? error,
    String? serverUrl,
    bool clearUser = false,
  }) {
    return AuthState(
      user: clearUser ? null : (user ?? this.user),
      isLoading: isLoading ?? this.isLoading,
      error: error,
      serverUrl: serverUrl ?? this.serverUrl,
    );
  }
}

class AuthNotifier extends Notifier<AuthState> {
  @override
  AuthState build() {
    final storage = ref.watch(storageServiceProvider);
    final url = kIsWeb ? Uri.base.origin : storage.getServerUrl();
    return AuthState(serverUrl: url);
  }

  Future<void> checkAuth() async {
    state = state.copyWith(isLoading: true, error: null);
    final authRepo = ref.read(authRepositoryProvider);
    try {
      final isAuthed = await authRepo.checkInitialAuth();
      if (isAuthed) {
        state = state.copyWith(
          user: const User(id: 'current', username: 'Connected', role: 'user'),
          isLoading: false,
        );
      } else {
        state = state.copyWith(clearUser: true, isLoading: false);
      }
    } catch (e) {
      state = state.copyWith(clearUser: true, isLoading: false, error: e.toString());
    }
  }

  Future<bool> login(String serverUrl, String username, String password) async {
    final effectiveUrl = kIsWeb ? Uri.base.origin : serverUrl;
    state = state.copyWith(isLoading: true, error: null, serverUrl: effectiveUrl);
    final authRepo = ref.read(authRepositoryProvider);
    try {
      final user = await authRepo.login(effectiveUrl, username, password);
      state = state.copyWith(user: user, isLoading: false, serverUrl: effectiveUrl);
      return true;
    } catch (e) {
      state = state.copyWith(clearUser: true, isLoading: false, error: e.toString());
      return false;
    }
  }

  Future<void> logout() async {
    final authRepo = ref.read(authRepositoryProvider);
    await authRepo.logout();
    state = state.copyWith(clearUser: true, isLoading: false);
  }
}

final authProvider = NotifierProvider<AuthNotifier, AuthState>(AuthNotifier.new);

// Library State
class LibraryState {
  final List<Book> books;
  final List<Author> authors;
  final List<Genre> genres;
  final List<Series> series;
  final String activeFilter; // 'all', 'series', 'authors', 'unread'
  final bool isLoading;
  final String? error;

  const LibraryState({
    this.books = const [],
    this.authors = const [],
    this.genres = const [],
    this.series = const [],
    this.activeFilter = 'all',
    this.isLoading = false,
    this.error,
  });

  List<Book> get filteredBooks {
    switch (activeFilter) {
      case 'unread':
        return books.where((b) => b.readingProgress < 1.0).toList();
      case 'series':
        return books.where((b) => b.series != null).toList();
      default:
        return books;
    }
  }

  LibraryState copyWith({
    List<Book>? books,
    List<Author>? authors,
    List<Genre>? genres,
    List<Series>? series,
    String? activeFilter,
    bool? isLoading,
    String? error,
  }) {
    return LibraryState(
      books: books ?? this.books,
      authors: authors ?? this.authors,
      genres: genres ?? this.genres,
      series: series ?? this.series,
      activeFilter: activeFilter ?? this.activeFilter,
      isLoading: isLoading ?? this.isLoading,
      error: error,
    );
  }
}

class LibraryNotifier extends Notifier<LibraryState> {
  @override
  LibraryState build() {
    return const LibraryState();
  }

  Future<void> loadLibrary() async {
    state = state.copyWith(isLoading: true, error: null);
    final bookRepo = ref.read(bookRepositoryProvider);
    try {
      final books = await bookRepo.getBooks();
      final authors = await bookRepo.getAuthors();
      final genres = await bookRepo.getGenres();
      final series = await bookRepo.getSeries();
      state = state.copyWith(
        books: books,
        authors: authors,
        genres: genres,
        series: series,
        isLoading: false,
      );
    } catch (e) {
      state = state.copyWith(isLoading: false, error: e.toString());
    }
  }

  void setFilter(String filter) {
    state = state.copyWith(activeFilter: filter);
  }
}

final libraryProvider = NotifierProvider<LibraryNotifier, LibraryState>(LibraryNotifier.new);

// Reader Settings State
class ReaderSettings {
  final ReadingThemeMode themeMode;
  final double fontSize;
  final double lineHeight;
  final bool isSerif;

  const ReaderSettings({
    this.themeMode = ReadingThemeMode.bone,
    this.fontSize = 18.0,
    this.lineHeight = 1.6,
    this.isSerif = true,
  });

  ReaderSettings copyWith({
    ReadingThemeMode? themeMode,
    double? fontSize,
    double? lineHeight,
    bool? isSerif,
  }) {
    return ReaderSettings(
      themeMode: themeMode ?? this.themeMode,
      fontSize: fontSize ?? this.fontSize,
      lineHeight: lineHeight ?? this.lineHeight,
      isSerif: isSerif ?? this.isSerif,
    );
  }
}

class ReaderSettingsNotifier extends Notifier<ReaderSettings> {
  @override
  ReaderSettings build() {
    final storage = ref.watch(storageServiceProvider);
    final modeStr = storage.getThemeMode();
    final mode = ReadingThemeMode.values.firstWhere(
      (m) => m.name == modeStr,
      orElse: () => ReadingThemeMode.bone,
    );
    final size = storage.getFontSize();
    final height = storage.getLineHeight();
    final family = storage.getFontFamily();

    return ReaderSettings(
      themeMode: mode,
      fontSize: size,
      lineHeight: height,
      isSerif: family == 'serif',
    );
  }

  void setThemeMode(ReadingThemeMode mode) {
    final storage = ref.read(storageServiceProvider);
    storage.saveThemeMode(mode.name);
    state = state.copyWith(themeMode: mode);
  }

  void setFontSize(double size) {
    final storage = ref.read(storageServiceProvider);
    storage.saveFontSize(size);
    state = state.copyWith(fontSize: size);
  }

  void setLineHeight(double height) {
    final storage = ref.read(storageServiceProvider);
    storage.saveLineHeight(height);
    state = state.copyWith(lineHeight: height);
  }

  void setFontFamily(bool isSerif) {
    final storage = ref.read(storageServiceProvider);
    storage.saveFontFamily(isSerif ? 'serif' : 'sans');
    state = state.copyWith(isSerif: isSerif);
  }
}

final readerSettingsProvider =
    NotifierProvider<ReaderSettingsNotifier, ReaderSettings>(ReaderSettingsNotifier.new);

// Semantic Search State
class SearchState {
  final String query;
  final List<SemanticSearchHit> results;
  final bool isLoading;
  final String? error;

  const SearchState({
    this.query = '',
    this.results = const [],
    this.isLoading = false,
    this.error,
  });

  SearchState copyWith({
    String? query,
    List<SemanticSearchHit>? results,
    bool? isLoading,
    String? error,
  }) {
    return SearchState(
      query: query ?? this.query,
      results: results ?? this.results,
      isLoading: isLoading ?? this.isLoading,
      error: error,
    );
  }
}

class SearchNotifier extends Notifier<SearchState> {
  @override
  SearchState build() {
    return const SearchState();
  }

  Future<void> search(String query) async {
    if (query.trim().isEmpty) {
      state = state.copyWith(query: '', results: [], isLoading: false);
      return;
    }

    state = state.copyWith(query: query, isLoading: true, error: null);
    final apiService = ref.read(apiServiceProvider);
    try {
      final hits = await apiService.semanticSearch(query);
      state = state.copyWith(results: hits, isLoading: false);
    } catch (e) {
      state = state.copyWith(isLoading: false, error: e.toString());
    }
  }
}

final searchProvider = NotifierProvider<SearchNotifier, SearchState>(SearchNotifier.new);

// Background Queue State & Notifier
class QueueState {
  final QueueStatus? status;
  final bool isLoading;
  final String? error;

  const QueueState({
    this.status,
    this.isLoading = false,
    this.error,
  });

  QueueState copyWith({
    QueueStatus? status,
    bool? isLoading,
    String? error,
  }) {
    return QueueState(
      status: status ?? this.status,
      isLoading: isLoading ?? this.isLoading,
      error: error,
    );
  }
}

class QueueNotifier extends Notifier<QueueState> {
  StreamSubscription<QueueStatus>? _subscription;
  Timer? _reconnectTimer;
  bool _isDisposed = false;

  @override
  QueueState build() {
    ref.onDispose(() {
      _isDisposed = true;
      _subscription?.cancel();
      _reconnectTimer?.cancel();
    });

    Future.microtask(() => _connectStream());

    return const QueueState(isLoading: true);
  }

  void _connectStream() {
    if (_isDisposed) return;
    _subscription?.cancel();
    _reconnectTimer?.cancel();

    final apiService = ref.read(apiServiceProvider);
    final storage = ref.read(storageServiceProvider);
    final token = apiService.token ?? storage.getAuthToken();
    if (token == null || token.isEmpty) {
      state = const QueueState(isLoading: false);
      _scheduleReconnect(const Duration(seconds: 10));
      return;
    }

    if (apiService.token == null || apiService.token!.isEmpty) {
      apiService.updateConnection(newToken: token);
    }

    try {
      _subscription = apiService.streamQueueEvents().listen(
        (status) {
          state = QueueState(status: status, isLoading: false);
        },
        onError: (err) {
          debugPrint('Queue SSE stream error: $err');
          refresh();
          _scheduleReconnect(const Duration(seconds: 5));
        },
        onDone: () {
          _scheduleReconnect(const Duration(seconds: 3));
        },
        cancelOnError: true,
      );
    } catch (e) {
      debugPrint('Error initiating Queue SSE stream: $e');
      refresh();
      _scheduleReconnect(const Duration(seconds: 5));
    }
  }

  void _scheduleReconnect(Duration delay) {
    if (_isDisposed) return;
    _reconnectTimer?.cancel();
    _reconnectTimer = Timer(delay, () {
      if (!_isDisposed) {
        _connectStream();
      }
    });
  }

  Future<void> refresh() async {
    final apiService = ref.read(apiServiceProvider);
    try {
      final status = await apiService.getQueueStatus();
      state = QueueState(status: status, isLoading: false);
    } catch (e) {
      state = state.copyWith(isLoading: false, error: e.toString());
    }
  }
}

final queueProvider = NotifierProvider<QueueNotifier, QueueState>(QueueNotifier.new);

// Book Detail State
class BookDetailState {
  final Book? book;
  final bool isLoading;
  final String? error;

  const BookDetailState({
    this.book,
    this.isLoading = false,
    this.error,
  });

  BookDetailState copyWith({
    Book? book,
    bool? isLoading,
    String? error,
  }) {
    return BookDetailState(
      book: book ?? this.book,
      isLoading: isLoading ?? this.isLoading,
      error: error,
    );
  }
}

class BookDetailNotifier extends Notifier<BookDetailState> {
  final String bookId;
  BookDetailNotifier(this.bookId);

  @override
  BookDetailState build() {
    Future.microtask(() => loadBook());
    return const BookDetailState(isLoading: true);
  }

  Future<void> loadBook() async {
    try {
      final repo = ref.read(bookRepositoryProvider);
      final book = await repo.getBookDetail(bookId);
      state = BookDetailState(book: book, isLoading: false);
    } catch (e) {
      state = BookDetailState(isLoading: false, error: e.toString());
    }
  }

  Future<void> addBookmark({
    required String title,
    double progress = 0.0,
    String? chapterId,
  }) async {
    try {
      final repo = ref.read(bookRepositoryProvider);
      final bm = await repo.createBookmark(
        bookId,
        title: title,
        progress: progress,
        chapterId: chapterId,
      );
      if (state.book != null) {
        final updatedBookmarks = [bm, ...state.book!.bookmarks];
        state = state.copyWith(book: state.book!.copyWith(bookmarks: updatedBookmarks));
      }
    } catch (e) {
      debugPrint('addBookmark error: $e');
    }
  }

  Future<void> removeBookmark(String bookmarkId) async {
    try {
      final repo = ref.read(bookRepositoryProvider);
      await repo.deleteBookmark(bookmarkId, bookId: bookId);
      if (state.book != null) {
        final updatedBookmarks = state.book!.bookmarks.where((b) => b.id != bookmarkId).toList();
        state = state.copyWith(book: state.book!.copyWith(bookmarks: updatedBookmarks));
      }
    } catch (e) {
      debugPrint('removeBookmark error: $e');
    }
  }

  Future<void> addHighlight({
    required String selectedText,
    String color = 'yellow',
    String? note,
    String? chapterId,
  }) async {
    try {
      final repo = ref.read(bookRepositoryProvider);
      final hl = await repo.createHighlight(
        bookId,
        selectedText: selectedText,
        color: color,
        note: note,
        chapterId: chapterId,
      );
      if (state.book != null) {
        final updatedHighlights = [hl, ...state.book!.highlights];
        state = state.copyWith(book: state.book!.copyWith(highlights: updatedHighlights));
      }
    } catch (e) {
      debugPrint('addHighlight error: $e');
    }
  }

  Future<void> removeHighlight(String highlightId) async {
    try {
      final repo = ref.read(bookRepositoryProvider);
      await repo.deleteHighlight(highlightId, bookId: bookId);
      if (state.book != null) {
        final updatedHighlights = state.book!.highlights.where((h) => h.id != highlightId).toList();
        state = state.copyWith(book: state.book!.copyWith(highlights: updatedHighlights));
      }
    } catch (e) {
      debugPrint('removeHighlight error: $e');
    }
  }
}

final bookDetailProvider =
    NotifierProvider.family<BookDetailNotifier, BookDetailState, String>(
  BookDetailNotifier.new,
);

// Book Chat State
class BookChatState {
  final List<BookChatMessage> messages;
  final bool isSending;
  final String? error;

  const BookChatState({
    this.messages = const [],
    this.isSending = false,
    this.error,
  });

  BookChatState copyWith({
    List<BookChatMessage>? messages,
    bool? isSending,
    String? error,
  }) {
    return BookChatState(
      messages: messages ?? this.messages,
      isSending: isSending ?? this.isSending,
      error: error,
    );
  }
}

class BookChatNotifier extends Notifier<BookChatState> {
  final String bookId;
  BookChatNotifier(this.bookId);

  @override
  BookChatState build() {
    return const BookChatState();
  }

  Future<void> sendMessage(String text) async {
    final query = text.trim();
    if (query.isEmpty) return;

    final userMsg = BookChatMessage(
      role: 'user',
      content: query,
      timestamp: DateTime.now(),
    );

    final currentMessages = [...state.messages, userMsg];
    state = state.copyWith(
      messages: currentMessages,
      isSending: true,
      error: null,
    );

    try {
      final repo = ref.read(bookRepositoryProvider);
      final history = currentMessages
          .take(currentMessages.length - 1)
          .map((m) => {'role': m.role, 'content': m.content})
          .toList();

      final res = await repo.chatWithBook(bookId, query, history: history);
      final assistantMsg = BookChatMessage(
        role: 'assistant',
        content: res.reply,
        citations: res.citations,
        timestamp: DateTime.now(),
      );

      state = state.copyWith(
        messages: [...currentMessages, assistantMsg],
        isSending: false,
      );
    } catch (e) {
      state = state.copyWith(
        isSending: false,
        error: e.toString(),
      );
    }
  }

  void clearChat() {
    state = const BookChatState();
  }
}

final bookChatProvider =
    NotifierProvider.family<BookChatNotifier, BookChatState, String>(
  BookChatNotifier.new,
);

