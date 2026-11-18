import 'dart:async';
import 'dart:convert';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../data/models/yoto.dart';
import '../../data/repositories/yoto_repository.dart';
import '../../data/services/api_service.dart';
import 'providers.dart';

final yotoRepositoryProvider = Provider<YotoRepository>((ref) {
  return YotoRepository(
    apiService: ref.watch(apiServiceProvider),
    storageService: ref.watch(storageServiceProvider),
  );
});

class YotoAccountState {
  final YotoStatus status;
  final bool isLoading;
  final String? errorMessage;
  final String? successMessage;

  const YotoAccountState({
    this.status = const YotoStatus(),
    this.isLoading = false,
    this.errorMessage,
    this.successMessage,
  });

  bool get isLinked => status.isLinked;

  YotoAccountState copyWith({
    YotoStatus? status,
    bool? isLoading,
    String? errorMessage,
    String? successMessage,
  }) {
    return YotoAccountState(
      status: status ?? this.status,
      isLoading: isLoading ?? this.isLoading,
      errorMessage: errorMessage,
      successMessage: successMessage,
    );
  }
}

class YotoAccountNotifier extends Notifier<YotoAccountState> {
  bool _isDisposed = false;

  @override
  YotoAccountState build() {
    _isDisposed = false;
    ref.onDispose(() {
      _isDisposed = true;
    });
    ref.listen(authProvider, (previous, next) {
      if (previous?.user != next.user && next.user != null) {
        loadStatus();
      }
    });
    Future.microtask(loadStatus);
    return const YotoAccountState(isLoading: true);
  }

  Future<void> _clearAuthOn401(Object e) async {
    if (e is ApiException && e.statusCode == 401) {
      await ref.read(storageServiceProvider).clearAuthToken();
      ref.read(apiServiceProvider).updateConnection(newToken: '');
    }
  }

  Future<void> loadStatus() async {
    if (_isDisposed) return;
    state = state.copyWith(isLoading: true, errorMessage: null);
    try {
      final repo = ref.read(yotoRepositoryProvider);
      final status = await repo.getStatus();
      if (!_isDisposed) {
        state = state.copyWith(status: status, isLoading: false);
      }
    } catch (e) {
      await _clearAuthOn401(e);
      if (!_isDisposed) {
        state = state.copyWith(
          isLoading: false,
          errorMessage: 'Failed to load Yoto account status: $e',
        );
      }
    }
  }

  Future<String> buildAuthUrl({
    required String clientId,
    required String redirectUri,
  }) async {
    final repo = ref.read(yotoRepositoryProvider);
    return repo.buildAuthorizationUrl(
      clientId: clientId,
      redirectUri: redirectUri,
      authBaseUrl: state.status.authBaseUrl,
    );
  }

  Future<bool> exchangeCode({
    required String code,
    String? clientId,
    String? redirectUri,
  }) async {
    state = state.copyWith(
      isLoading: true,
      errorMessage: null,
      successMessage: null,
    );
    try {
      final repo = ref.read(yotoRepositoryProvider);
      final status = await repo.exchangeOAuthCode(
        code: code,
        clientId: clientId,
        redirectUri: redirectUri,
      );
      state = state.copyWith(
        status: status,
        isLoading: false,
        successMessage: 'Yoto account linked successfully!',
      );
      return true;
    } catch (e) {
      await _clearAuthOn401(e);
      state = state.copyWith(
        isLoading: false,
        errorMessage: 'Failed to link Yoto account: $e',
      );
      return false;
    }
  }

  Future<bool> linkWithTokens({
    required String accessToken,
    String? refreshToken,
    String? clientId,
  }) async {
    state = state.copyWith(
      isLoading: true,
      errorMessage: null,
      successMessage: null,
    );
    try {
      final repo = ref.read(yotoRepositoryProvider);
      final status = await repo.linkWithTokens(
        accessToken: accessToken,
        refreshToken: refreshToken,
        clientId: clientId,
      );
      state = state.copyWith(
        status: status,
        isLoading: false,
        successMessage: 'Yoto account linked successfully!',
      );
      return true;
    } catch (e) {
      await _clearAuthOn401(e);
      state = state.copyWith(
        isLoading: false,
        errorMessage: 'Failed to link Yoto account: $e',
      );
      return false;
    }
  }

  Future<void> unlink() async {
    state = state.copyWith(
      isLoading: true,
      errorMessage: null,
      successMessage: null,
    );
    try {
      final repo = ref.read(yotoRepositoryProvider);
      await repo.unlinkAccount();
      state = state.copyWith(
        status: YotoStatus(
          isLinked: false,
          defaultClientId: state.status.defaultClientId,
          authBaseUrl: state.status.authBaseUrl,
        ),
        isLoading: false,
        successMessage: 'Yoto account unlinked.',
      );
    } catch (e) {
      await _clearAuthOn401(e);
      state = state.copyWith(
        isLoading: false,
        errorMessage: 'Failed to unlink Yoto account: $e',
      );
    }
  }
}

final yotoAccountProvider =
    NotifierProvider<YotoAccountNotifier, YotoAccountState>(
  YotoAccountNotifier.new,
);

final yotoPlaylistsProvider = FutureProvider.autoDispose<List<YotoPlaylist>>((ref) async {
  final repo = ref.watch(yotoRepositoryProvider);
  return repo.getPlaylists();
});

final yotoIconsProvider = FutureProvider.autoDispose<YotoIconsCatalog>((ref) async {
  final repo = ref.watch(yotoRepositoryProvider);
  return repo.getIcons();
});

enum YotoUploadTaskStatus { pending, uploading, completed, failed }

class YotoUploadTask {
  final String id;
  final String title;
  final String? bookId;
  final String? seriesId;
  final SendToYotoRequest request;
  final YotoUploadTaskStatus status;
  final double currentProgress;
  final double overallProgress;
  final String? currentItemLabel;
  final String? stage;
  final int subCompletedItems;
  final int subTotalItems;
  final String? errorMessage;

  const YotoUploadTask({
    required this.id,
    required this.title,
    this.bookId,
    this.seriesId,
    required this.request,
    this.status = YotoUploadTaskStatus.pending,
    this.currentProgress = 0.0,
    this.overallProgress = 0.0,
    this.currentItemLabel,
    this.stage,
    this.subCompletedItems = 0,
    this.subTotalItems = 1,
    this.errorMessage,
  });

  bool get isSeries => seriesId != null && seriesId!.isNotEmpty;

  bool get isFinished =>
      status == YotoUploadTaskStatus.completed ||
      status == YotoUploadTaskStatus.failed;

  YotoUploadTask copyWith({
    YotoUploadTaskStatus? status,
    double? currentProgress,
    double? overallProgress,
    String? currentItemLabel,
    String? stage,
    int? subCompletedItems,
    int? subTotalItems,
    String? errorMessage,
  }) {
    return YotoUploadTask(
      id: id,
      title: title,
      bookId: bookId,
      seriesId: seriesId,
      request: request,
      status: status ?? this.status,
      currentProgress: currentProgress ?? this.currentProgress,
      overallProgress: overallProgress ?? this.overallProgress,
      currentItemLabel: currentItemLabel ?? this.currentItemLabel,
      stage: stage ?? this.stage,
      subCompletedItems: subCompletedItems ?? this.subCompletedItems,
      subTotalItems: subTotalItems ?? this.subTotalItems,
      errorMessage: errorMessage ?? this.errorMessage,
    );
  }
}

class YotoUploadErrorEvent {
  final int id;
  final String title;
  final String message;

  const YotoUploadErrorEvent({
    required this.id,
    required this.title,
    required this.message,
  });
}

class YotoUploadQueueState {
  final List<YotoUploadTask> tasks;
  final bool isProcessing;
  final YotoUploadErrorEvent? latestError;

  const YotoUploadQueueState({
    this.tasks = const [],
    this.isProcessing = false,
    this.latestError,
  });

  YotoUploadTask? get activeTask {
    for (final t in tasks) {
      if (t.status == YotoUploadTaskStatus.uploading) {
        return t;
      }
    }
    for (final t in tasks) {
      if (t.status == YotoUploadTaskStatus.pending) {
        return t;
      }
    }
    return null;
  }

  bool get hasActiveOrPending =>
      isProcessing ||
      tasks.any(
        (t) =>
            t.status == YotoUploadTaskStatus.uploading ||
            t.status == YotoUploadTaskStatus.pending,
      );

  /// Progress (0.0 - 1.0) of the currently running upload.
  double get currentUploadProgress {
    final active = activeTask;
    if (active == null) return 0.0;
    return active.currentProgress.clamp(0.0, 1.0);
  }

  /// Total number of upload units in the active batch.
  int get totalUploadsCount {
    if (tasks.isEmpty) return 0;
    if (tasks.length == 1 && tasks.first.subTotalItems > 1) {
      return tasks.first.subTotalItems;
    }
    return tasks.length;
  }

  /// Completed number of upload units in the active batch.
  int get completedUploadsCount {
    if (tasks.isEmpty) return 0;
    if (tasks.length == 1 && tasks.first.subTotalItems > 1) {
      return tasks.first.subCompletedItems.clamp(0, tasks.first.subTotalItems);
    }
    return tasks.where((t) => t.isFinished).length;
  }

  /// Progress (0.0 - 1.0) across all uploads in the active batch.
  double get totalUploadsProgress {
    if (tasks.isEmpty) return 0.0;
    if (tasks.length == 1 && tasks.first.subTotalItems > 1) {
      return tasks.first.overallProgress.clamp(0.0, 1.0);
    }
    final completed = tasks.where((t) => t.isFinished).length;
    final active = activeTask;
    final activeOverall = active != null ? active.overallProgress.clamp(0.0, 1.0) : 0.0;
    return ((completed + activeOverall) / tasks.length).clamp(0.0, 1.0);
  }

  String get currentUploadTitle {
    final active = activeTask;
    if (active == null) return 'Uploading to Yoto...';
    if (active.currentItemLabel != null &&
        active.currentItemLabel!.isNotEmpty &&
        active.currentItemLabel != active.title) {
      return '${active.title} • ${active.currentItemLabel}';
    }
    return active.title;
  }

  String get currentStageDescription {
    final active = activeTask;
    if (active == null) return 'Queued';
    switch (active.stage) {
      case 'uploading':
        return 'Uploading audio...';
      case 'transcoding':
        return 'Transcoding on Yoto...';
      case 'saving':
        return 'Saving playlist...';
      default:
        return active.status == YotoUploadTaskStatus.uploading
            ? 'Uploading & transcoding...'
            : 'Queued';
    }
  }

  YotoUploadQueueState copyWith({
    List<YotoUploadTask>? tasks,
    bool? isProcessing,
    YotoUploadErrorEvent? latestError,
    bool clearError = false,
  }) {
    return YotoUploadQueueState(
      tasks: tasks ?? this.tasks,
      isProcessing: isProcessing ?? this.isProcessing,
      latestError: clearError ? null : (latestError ?? this.latestError),
    );
  }
}

class YotoUploadQueueNotifier extends Notifier<YotoUploadQueueState> {
  bool _isDisposed = false;
  int _nextTaskSeq = 1;
  int _nextErrorSeq = 1;
  Timer? _pollTimer;

  @override
  YotoUploadQueueState build() {
    _isDisposed = false;
    ref.onDispose(() {
      _isDisposed = true;
      _pollTimer?.cancel();
      _pollTimer = null;
    });
    return const YotoUploadQueueState();
  }

  void enqueueBookUpload({
    required String bookId,
    required String title,
    required SendToYotoRequest request,
  }) {
    final task = YotoUploadTask(
      id: 'yoto-book-${_nextTaskSeq++}',
      title: title,
      bookId: bookId,
      request: request,
      subTotalItems: 1,
    );
    _enqueueTask(task);
  }

  void enqueueSeriesUpload({
    required String seriesId,
    required String title,
    required SendToYotoRequest request,
    int bookCount = 1,
  }) {
    final task = YotoUploadTask(
      id: 'yoto-series-${_nextTaskSeq++}',
      title: title,
      seriesId: seriesId,
      request: request,
      subTotalItems: bookCount > 0 ? bookCount : 1,
    );
    _enqueueTask(task);
  }

  void _enqueueTask(YotoUploadTask task) {
    if (_isDisposed) return;
    // If previous batch already finished completely, start a fresh batch list
    final existing = state.hasActiveOrPending
        ? state.tasks
        : const <YotoUploadTask>[];
    state = state.copyWith(
      tasks: [...existing, task],
    );
    if (!state.isProcessing) {
      unawaited(_processQueue());
    }
  }

  void acknowledgeError(int errorId) {
    if (_isDisposed) return;
    if (state.latestError?.id == errorId) {
      state = state.copyWith(clearError: true);
    }
  }

  void _updateTask(String taskId, YotoUploadTask Function(YotoUploadTask) updater) {
    if (_isDisposed) return;
    final updated = state.tasks
        .map((t) => t.id == taskId ? updater(t) : t)
        .toList(growable: false);
    state = state.copyWith(tasks: updated);
  }

  String _formatErrorMessage(String title, Object error) {
    String detail = error.toString();
    if (error is ApiException) {
      try {
        final decoded = jsonDecode(error.message);
        if (decoded is Map<String, dynamic> && decoded['error'] != null) {
          detail = decoded['error'].toString();
        } else if (error.message.trim().isNotEmpty) {
          detail = error.message.trim();
        }
      } catch (_) {
        if (error.message.trim().isNotEmpty) {
          detail = error.message.trim();
        }
      }
    }
    return 'Yoto upload failed for "$title": $detail';
  }

  Future<void> _processQueue() async {
    if (_isDisposed || state.isProcessing) return;
    state = state.copyWith(isProcessing: true);

    try {
      while (!_isDisposed) {
        final nextIdx = state.tasks.indexWhere(
          (t) => t.status == YotoUploadTaskStatus.pending,
        );
        if (nextIdx == -1) {
          break;
        }

        final task = state.tasks[nextIdx];
        _updateTask(
          task.id,
          (t) => t.copyWith(
            status: YotoUploadTaskStatus.uploading,
            currentProgress: 0.08,
            overallProgress: 0.08 / (t.subTotalItems > 0 ? t.subTotalItems : 1),
            stage: 'uploading',
          ),
        );

        _startProgressPolling(task.id);
        try {
          final repo = ref.read(yotoRepositoryProvider);
          if (task.isSeries) {
            await repo.sendSeriesToYoto(task.seriesId!, task.request);
          } else {
            await repo.sendBookToYoto(task.bookId!, task.request);
          }
          _stopProgressPolling();
          if (_isDisposed) return;

          _updateTask(
            task.id,
            (t) => t.copyWith(
              status: YotoUploadTaskStatus.completed,
              currentProgress: 1.0,
              overallProgress: 1.0,
              subCompletedItems: t.subTotalItems,
              stage: 'completed',
            ),
          );
          ref.invalidate(yotoPlaylistsProvider);
        } catch (e) {
          _stopProgressPolling();
          if (_isDisposed) return;

          final errMsg = _formatErrorMessage(task.title, e);
          _updateTask(
            task.id,
            (t) => t.copyWith(
              status: YotoUploadTaskStatus.failed,
              currentProgress: 1.0,
              overallProgress: 1.0,
              errorMessage: errMsg,
            ),
          );
          state = state.copyWith(
            latestError: YotoUploadErrorEvent(
              id: _nextErrorSeq++,
              title: task.title,
              message: errMsg,
            ),
          );
        }
      }
    } finally {
      _stopProgressPolling();
      if (!_isDisposed) {
        // Clear completed batch when all tasks in the queue have finished
        state = state.copyWith(
          isProcessing: false,
          tasks: const [],
        );
      }
    }
  }

  Future<void> _pollProgressOnce(String taskId) async {
    if (_isDisposed) return;
    final repo = ref.read(yotoRepositoryProvider);
    final serverProg = await repo.getUploadProgress();
    if (_isDisposed) return;

    _updateTask(taskId, (t) {
      if (t.status != YotoUploadTaskStatus.uploading) return t;
      if (serverProg.isActive) {
        final effCurrent = serverProg.currentProgress > t.currentProgress
            ? serverProg.currentProgress
            : t.currentProgress;
        final effOverall = serverProg.overallProgress > t.overallProgress
            ? serverProg.overallProgress
            : t.overallProgress;
        return t.copyWith(
          currentProgress: effCurrent.clamp(0.05, 0.98),
          overallProgress: effOverall.clamp(0.05, 0.98),
          currentItemLabel: serverProg.currentItem,
          stage: serverProg.stage,
          subCompletedItems: serverProg.completedItems,
          subTotalItems: serverProg.totalItems > 0
              ? serverProg.totalItems
              : t.subTotalItems,
        );
      }
      // Smooth fallback step while awaiting server response
      final nextCurrent = (t.currentProgress + 0.06).clamp(0.08, 0.90);
      final nextOverall =
          (t.overallProgress + 0.06 / (t.subTotalItems > 0 ? t.subTotalItems : 1))
              .clamp(0.08, 0.90);
      return t.copyWith(
        currentProgress: nextCurrent,
        overallProgress: nextOverall,
      );
    });
  }

  void _startProgressPolling(String taskId) {
    _stopProgressPolling();
    unawaited(_pollProgressOnce(taskId));
    _pollTimer = Timer.periodic(const Duration(milliseconds: 400), (_) {
      unawaited(_pollProgressOnce(taskId));
    });
  }

  void _stopProgressPolling() {
    _pollTimer?.cancel();
    _pollTimer = null;
  }
}

final yotoUploadQueueProvider =
    NotifierProvider<YotoUploadQueueNotifier, YotoUploadQueueState>(
  YotoUploadQueueNotifier.new,
);
