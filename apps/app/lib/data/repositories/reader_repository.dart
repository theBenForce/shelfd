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

  Future<Chapter> loadChapter(String bookId, int chapterIndex) async {
    final cached = storageService.getCachedChapter(bookId, chapterIndex);
    if (cached != null && cached.isNotEmpty) {
      return Chapter(
        id: '${bookId}_$chapterIndex',
        bookId: bookId,
        chapterIndex: chapterIndex,
        title: 'Chapter $chapterIndex',
        content: cached,
      );
    }

    final chapter = await apiService.getChapter(bookId, chapterIndex);
    if (chapter.content != null) {
      await storageService.cacheChapter(bookId, chapterIndex, chapter.content!);
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
