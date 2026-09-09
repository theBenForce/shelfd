import '../models/chapter.dart';
import '../services/api_service.dart';
import '../services/storage_service.dart';

class ReaderRepository {
  final ApiService apiService;
  final StorageService storageService;

  ReaderRepository({
    required this.apiService,
    required this.storageService,
  });

  Future<Chapter> loadChapter(String bookId, dynamic chapterIdentifier) async {
    final cachedData = storageService.getCachedChapterData(bookId, chapterIdentifier);
    if (cachedData != null) {
      return Chapter.fromJson(cachedData);
    }

    final cached = storageService.getCachedChapter(bookId, chapterIdentifier);
    if (cached != null && cached.isNotEmpty) {
      final index = chapterIdentifier is int ? chapterIdentifier : 0;
      final fallbackTitle = chapterIdentifier is int ? 'Chapter $chapterIdentifier' : '';
      return Chapter(
        id: '${bookId}_$chapterIdentifier',
        bookId: bookId,
        chapterIndex: index,
        title: fallbackTitle,
        content: cached,
      );
    }

    final chapter = await apiService.getChapter(bookId, chapterIdentifier);
    if (chapter.content != null) {
      await storageService.cacheChapterData(bookId, chapterIdentifier, chapter.toJson());
      if (chapterIdentifier != chapter.id && chapter.id.isNotEmpty) {
        await storageService.cacheChapterData(bookId, chapter.id, chapter.toJson());
      }
    }
    return chapter;
  }

  Future<void> updateProgress(String bookId, double progress) async {
    await storageService.saveReadingProgress(bookId, progress);
  }

  double getProgress(String bookId) {
    return storageService.getReadingProgress(bookId);
  }
}
