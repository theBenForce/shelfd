import '../models/audiobook.dart';
import '../services/api_service.dart';
import '../services/storage_service.dart';

class AudioRepository {
  final ApiService apiService;
  final StorageService storageService;

  AudioRepository({
    required this.apiService,
    required this.storageService,
  });

  Future<List<AudioChapter>> getChapters(String bookId) async {
    try {
      return await apiService.getAudioChapters(bookId);
    } catch (_) {
      return [];
    }
  }

  Future<AudiobookProgress> getProgress(String bookId) async {
    try {
      final serverProgress = await apiService.getAudiobookProgress(bookId);
      await storageService.saveAudiobookProgress(bookId, serverProgress.positionSeconds);
      await storageService.saveAudiobookSpeed(bookId, serverProgress.speed);
      return serverProgress;
    } catch (_) {
      final localPos = storageService.getAudiobookProgress(bookId);
      final localSpeed = storageService.getAudiobookSpeed(bookId);
      return AudiobookProgress(
        bookId: bookId,
        positionSeconds: localPos,
        speed: localSpeed,
        isCompleted: false,
      );
    }
  }

  Future<AudiobookProgress> saveProgress(
    String bookId, {
    required double positionSeconds,
    double speed = 1.0,
    bool isCompleted = false,
  }) async {
    await storageService.saveAudiobookProgress(bookId, positionSeconds);
    await storageService.saveAudiobookSpeed(bookId, speed);
    try {
      return await apiService.saveAudiobookProgress(
        bookId,
        positionSeconds: positionSeconds,
        speed: speed,
        isCompleted: isCompleted,
      );
    } catch (_) {
      return AudiobookProgress(
        bookId: bookId,
        positionSeconds: positionSeconds,
        speed: speed,
        isCompleted: isCompleted,
      );
    }
  }

  Future<void> deleteProgress(String bookId) async {
    await storageService.saveAudiobookProgress(bookId, 0.0);
    try {
      await apiService.deleteAudiobookProgress(bookId);
    } catch (_) {}
  }

  String getStreamUrl(String bookId) {
    return apiService.getAudioStreamUrl(bookId);
  }

  String getFileStreamUrl(String bookId, String fileId) {
    return apiService.getBookFileStreamUrl(bookId, fileId);
  }
}
