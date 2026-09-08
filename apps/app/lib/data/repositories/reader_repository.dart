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
    final cached = storageService.getCachedChapter(bookId, chapterIdentifier);
    if (cached != null && cached.isNotEmpty) {
      final index = chapterIdentifier is int ? chapterIdentifier : 0;
      return Chapter(
        id: '${bookId}_$chapterIdentifier',
        bookId: bookId,
        chapterIndex: index,
        title: 'Chapter $chapterIdentifier',
        content: cached,
      );
    }

    final chapter = await apiService.getChapter(bookId, chapterIdentifier);
    if (chapter.content != null) {
      await storageService.cacheChapter(bookId, chapterIdentifier, chapter.content!);
      if (chapterIdentifier != chapter.id && chapter.id.isNotEmpty) {
        await storageService.cacheChapter(bookId, chapter.id, chapter.content!);
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
