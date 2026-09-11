import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../data/models/book.dart';
import '../../data/models/upload_job.dart';
import 'providers.dart';

class PickedEpubFile {
  final String name;
  final String? path;
  final List<int>? bytes;
  final Future<List<int>> Function()? readBytes;

  const PickedEpubFile({
    required this.name,
    this.path,
    this.bytes,
    this.readBytes,
  });

  Future<List<int>> getBytes() async {
    if (bytes != null && bytes!.isNotEmpty) return bytes!;
    if (readBytes != null) return await readBytes!();
    return const [];
  }
}

class UploadState {
  final bool isUploading;
  final double? uploadProgress; // 0.0 to 1.0
  final String? currentBatchStatus;
  final List<StagedUploadJob> stagedJobs;
  final String? selectedJobId;
  final bool isLoadingJobs;
  final bool autoCommit;
  final String? error;

  const UploadState({
    this.isUploading = false,
    this.uploadProgress,
    this.currentBatchStatus,
    this.stagedJobs = const [],
    this.selectedJobId,
    this.isLoadingJobs = false,
    this.autoCommit = false,
    this.error,
  });

  StagedUploadJob? get selectedJob {
    if (stagedJobs.isEmpty) return null;
    if (selectedJobId == null) return stagedJobs.first;
    return stagedJobs.firstWhere(
      (j) => j.jobId == selectedJobId,
      orElse: () => stagedJobs.first,
    );
  }

  int get readyCount => stagedJobs.where((j) => j.warnings.isEmpty).length;
  int get warningsCount => stagedJobs.where((j) => j.warnings.isNotEmpty).length;

  UploadState copyWith({
    bool? isUploading,
    double? uploadProgress,
    bool clearUploadProgress = false,
    String? currentBatchStatus,
    bool clearCurrentBatchStatus = false,
    List<StagedUploadJob>? stagedJobs,
    String? selectedJobId,
    bool clearSelectedJobId = false,
    bool? isLoadingJobs,
    bool? autoCommit,
    String? error,
    bool clearError = false,
  }) {
    return UploadState(
      isUploading: isUploading ?? this.isUploading,
      uploadProgress: clearUploadProgress ? null : (uploadProgress ?? this.uploadProgress),
      currentBatchStatus: clearCurrentBatchStatus ? null : (currentBatchStatus ?? this.currentBatchStatus),
      stagedJobs: stagedJobs ?? this.stagedJobs,
      selectedJobId: clearSelectedJobId ? null : (selectedJobId ?? this.selectedJobId),
      isLoadingJobs: isLoadingJobs ?? this.isLoadingJobs,
      autoCommit: autoCommit ?? this.autoCommit,
      error: clearError ? null : (error ?? this.error),
    );
  }
}

class UploadNotifier extends Notifier<UploadState> {
  @override
  UploadState build() {
    final storage = ref.watch(storageServiceProvider);
    final autoCommit = storage.getAutoCommitUploads();

    // Listen to queueProvider for staged count changes
    ref.listen(queueProvider, (previous, next) {
      final prevStaged = previous?.status?.stagedUploads ?? 0;
      final nextStaged = next.status?.stagedUploads ?? 0;
      if (prevStaged != nextStaged && !state.isUploading && !state.isLoadingJobs) {
        loadStagedJobs();
      }
    });

    Future.microtask(() => loadStagedJobs());

    return UploadState(autoCommit: autoCommit);
  }

  Future<void> loadStagedJobs() async {
    state = state.copyWith(isLoadingJobs: true, clearError: true);
    final bookRepo = ref.read(bookRepositoryProvider);
    try {
      final jobs = await bookRepo.getStagedUploadJobs();
      final currentSelected = state.selectedJobId;
      String? nextSelected = currentSelected;
      if (jobs.isNotEmpty) {
        if (currentSelected == null || !jobs.any((j) => j.jobId == currentSelected)) {
          nextSelected = jobs.first.jobId;
        }
      } else {
        nextSelected = null;
      }
      state = state.copyWith(
        stagedJobs: jobs,
        selectedJobId: nextSelected,
        isLoadingJobs: false,
      );
    } catch (e) {
      state = state.copyWith(
        isLoadingJobs: false,
        error: e.toString(),
      );
    }
  }

  void selectJob(String? jobId) {
    state = state.copyWith(selectedJobId: jobId);
  }

  void updateStagedMetadata(String jobId, StagedMetadata metadata) {
    final index = state.stagedJobs.indexWhere((j) => j.jobId == jobId);
    if (index >= 0) {
      final updatedJobs = List<StagedUploadJob>.from(state.stagedJobs);
      final current = updatedJobs[index];
      final newWarnings = <String>[];
      if (metadata.authors.isEmpty ||
          (metadata.authors.length == 1 && metadata.authors[0].toLowerCase() == 'unknown')) {
        newWarnings.add('No author found in EPUB metadata');
      }
      if (metadata.title == 'Untitled' || metadata.title.isEmpty) {
        newWarnings.add('No title found in EPUB metadata');
      }
      updatedJobs[index] = current.copyWith(
        metadata: metadata,
        warnings: newWarnings,
      );
      state = state.copyWith(stagedJobs: updatedJobs);
    }
  }

  Future<void> toggleAutoCommit(bool enabled) async {
    final storage = ref.read(storageServiceProvider);
    await storage.saveAutoCommitUploads(enabled);
    state = state.copyWith(autoCommit: enabled);
  }

  Future<int> uploadEpubFiles(List<PickedEpubFile> files) async {
    if (files.isEmpty) return 0;

    state = state.copyWith(
      isUploading: true,
      uploadProgress: 0.0,
      clearError: true,
    );

    final bookRepo = ref.read(bookRepositoryProvider);
    final autoCommit = state.autoCommit;
    int successCount = 0;

    for (int i = 0; i < files.length; i++) {
      final file = files[i];
      final currentNum = i + 1;
      final total = files.length;
      final progress = i / total;

      state = state.copyWith(
        uploadProgress: progress,
        currentBatchStatus: 'Staging $currentNum of $total: ${file.name}',
      );

      try {
        final bytes = await file.getBytes();
        if (bytes.isEmpty) continue;

        final staged = await bookRepo.stageUpload(filename: file.name, bytes: bytes);
        successCount++;

        if (autoCommit) {
          state = state.copyWith(
            currentBatchStatus: 'Auto-committing $currentNum of $total: ${staged.metadata.title}',
          );
          final book = await bookRepo.commitUpload(staged.jobId, staged.metadata);
          ref.read(libraryProvider.notifier).addBook(book);
        }
      } catch (e) {
        debugPrint('Error uploading ${file.name}: $e');
      }
    }

    state = state.copyWith(
      isUploading: false,
      clearUploadProgress: true,
      clearCurrentBatchStatus: true,
    );

    await loadStagedJobs();
    ref.read(queueProvider.notifier).refresh();
    return successCount;
  }

  Future<Book?> commitJob(String jobId, StagedMetadata metadata) async {
    final bookRepo = ref.read(bookRepositoryProvider);
    try {
      final book = await bookRepo.commitUpload(jobId, metadata);
      ref.read(libraryProvider.notifier).addBook(book);

      final updatedJobs = state.stagedJobs.where((j) => j.jobId != jobId).toList();
      String? nextSelected;
      if (updatedJobs.isNotEmpty) {
        nextSelected = updatedJobs.first.jobId;
      }
      state = state.copyWith(
        stagedJobs: updatedJobs,
        selectedJobId: nextSelected,
      );

      ref.read(queueProvider.notifier).refresh();
      return book;
    } catch (e) {
      state = state.copyWith(error: 'Failed to commit upload: $e');
      return null;
    }
  }

  Future<int> commitAllReady() async {
    final readyJobs = state.stagedJobs.where((j) => j.warnings.isEmpty).toList();
    if (readyJobs.isEmpty) return 0;

    state = state.copyWith(isUploading: true, clearError: true);
    final bookRepo = ref.read(bookRepositoryProvider);
    int committed = 0;

    for (int i = 0; i < readyJobs.length; i++) {
      final job = readyJobs[i];
      state = state.copyWith(
        uploadProgress: i / readyJobs.length,
        currentBatchStatus: 'Committing ${i + 1} of ${readyJobs.length}: ${job.metadata.title}',
      );

      try {
        final book = await bookRepo.commitUpload(job.jobId, job.metadata);
        ref.read(libraryProvider.notifier).addBook(book);
        committed++;
      } catch (e) {
        debugPrint('Error batch committing ${job.jobId}: $e');
      }
    }

    state = state.copyWith(
      isUploading: false,
      clearUploadProgress: true,
      clearCurrentBatchStatus: true,
    );

    await loadStagedJobs();
    ref.read(queueProvider.notifier).refresh();
    return committed;
  }

  Future<void> discardJob(String jobId) async {
    final bookRepo = ref.read(bookRepositoryProvider);
    try {
      await bookRepo.deleteUploadJob(jobId);
      final updatedJobs = state.stagedJobs.where((j) => j.jobId != jobId).toList();
      String? nextSelected;
      if (updatedJobs.isNotEmpty) {
        nextSelected = updatedJobs.first.jobId;
      }
      state = state.copyWith(
        stagedJobs: updatedJobs,
        selectedJobId: nextSelected,
      );
      ref.read(queueProvider.notifier).refresh();
    } catch (e) {
      state = state.copyWith(error: 'Failed to discard upload: $e');
    }
  }

  Future<void> clearAllStaged() async {
    final bookRepo = ref.read(bookRepositoryProvider);
    state = state.copyWith(isUploading: true);
    for (final job in state.stagedJobs) {
      try {
        await bookRepo.deleteUploadJob(job.jobId);
      } catch (_) {}
    }
    state = state.copyWith(
      isUploading: false,
      stagedJobs: const [],
      clearSelectedJobId: true,
    );
    ref.read(queueProvider.notifier).refresh();
  }
}

final uploadProvider = NotifierProvider<UploadNotifier, UploadState>(UploadNotifier.new);
