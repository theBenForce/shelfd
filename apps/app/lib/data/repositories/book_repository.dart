import '../models/author.dart';
import '../models/book.dart';
import '../models/genre.dart';
import '../models/series.dart';
import '../services/api_service.dart';
import '../services/storage_service.dart';

class BookRepository {
  final ApiService apiService;
  final StorageService storageService;

  BookRepository({
    required this.apiService,
    required this.storageService,
  });

  Future<List<Book>> getBooks({
    int page = 1,
    int perPage = 20,
    String? authorId,
    String? genreId,
    String? seriesId,
    String? search,
  }) async {
    final books = await apiService.getBooks(
      page: page,
      perPage: perPage,
      authorId: authorId,
      genreId: genreId,
      seriesId: seriesId,
      search: search,
    );

    return books.map((book) {
      final progress = storageService.getReadingProgress(book.id);
      return book.copyWith(readingProgress: progress);
    }).toList();
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
}
