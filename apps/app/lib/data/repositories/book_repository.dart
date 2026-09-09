import '../models/author.dart';
import '../models/book.dart';
import '../models/book_chat.dart';
import '../models/bookmark.dart';
import '../models/genre.dart';
import '../models/highlight.dart';
import '../models/paginated_books.dart';
import '../models/series.dart';
import '../models/upload_job.dart';
import '../services/api_service.dart';
import '../services/storage_service.dart';

class BookRepository {
  final ApiService apiService;
  final StorageService storageService;

  BookRepository({
    required this.apiService,
    required this.storageService,
  });

  Future<PaginatedBooks> getBooksPage({
    int page = 1,
    int perPage = 24,
    String? authorId,
    String? genreId,
    String? seriesId,
    String? search,
  }) async {
    final paginated = await apiService.getBooksPage(
      page: page,
      perPage: perPage,
      authorId: authorId,
      genreId: genreId,
      seriesId: seriesId,
      search: search,
    );

    final hydratedBooks = paginated.books.map((book) {
      final progress = storageService.getReadingProgress(book.id);
      return book.copyWith(readingProgress: progress);
    }).toList();

    return PaginatedBooks(
      books: hydratedBooks,
      total: paginated.total,
      page: paginated.page,
      perPage: paginated.perPage,
      limit: paginated.limit,
      offset: paginated.offset,
    );
  }

  Future<List<Book>> getBooks({
    int page = 1,
    int perPage = 24,
    String? authorId,
    String? genreId,
    String? seriesId,
    String? search,
  }) async {
    final pageData = await getBooksPage(
      page: page,
      perPage: perPage,
      authorId: authorId,
      genreId: genreId,
      seriesId: seriesId,
      search: search,
    );
    return pageData.books;
  }

  Future<Book> getBookDetail(String id) async {
    final book = await apiService.getBook(id);
    final progress = storageService.getReadingProgress(book.id);
    return book.copyWith(readingProgress: progress);
  }

  Future<List<Author>> getAuthors() => apiService.getAuthors();
  Future<List<Genre>> getGenres() => apiService.getGenres();
  Future<List<Series>> getSeries() => apiService.getSeries();
  Future<void> triggerScan() => apiService.triggerLibraryScan();

  Future<Bookmark> createBookmark(
    String bookId, {
    required String title,
    double progress = 0.0,
    String? chapterId,
  }) =>
      apiService.createBookmark(
        bookId,
        title: title,
        progress: progress,
        chapterId: chapterId,
      );

  Future<List<Bookmark>> getBookmarks(String bookId) => apiService.getBookmarks(bookId);

  Future<void> deleteBookmark(String bookmarkId, {String? bookId}) =>
      apiService.deleteBookmark(bookmarkId, bookId: bookId);

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
  }) =>
      apiService.createHighlight(
        bookId,
        selectedText: selectedText,
        color: color,
        note: note,
        chapterId: chapterId,
        startOffset: startOffset,
        endOffset: endOffset,
        startParagraph: startParagraph,
        endParagraph: endParagraph,
        location: location,
      );

  Future<List<Highlight>> getHighlights(String bookId) => apiService.getHighlights(bookId);

  Future<void> deleteHighlight(String highlightId, {String? bookId}) =>
      apiService.deleteHighlight(highlightId, bookId: bookId);

  Future<BookChatResponse> chatWithBook(
    String bookId,
    String message, {
    List<Map<String, dynamic>>? history,
  }) =>
      apiService.chatWithBook(bookId, message, history: history);

  String getUploadJobCoverUrl(String jobId) => apiService.getUploadJobCoverUrl(jobId);

  Future<StagedUploadJob> stageUpload({
    required String filename,
    required List<int> bytes,
  }) =>
      apiService.stageUploadBook(filename: filename, bytes: bytes);

  Future<Book> commitUpload(String jobId, StagedMetadata metadata) =>
      apiService.commitUploadJob(jobId, metadata);

  Future<void> deleteUploadJob(String jobId) =>
      apiService.deleteUploadJob(jobId);
}
