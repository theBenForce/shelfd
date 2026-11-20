import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../data/models/background_job.dart';
import 'providers.dart';

class BackgroundJobsState {
  final List<BackgroundJob> jobs;
  final List<BackgroundJob> unacknowledgedErrors;
  final bool isLoading;

  const BackgroundJobsState({
    this.jobs = const [],
    this.unacknowledgedErrors = const [],
    this.isLoading = false,
  });

  List<BackgroundJob> get activeJobs =>
      jobs.where((j) => j.isActive).toList(growable: false);

  List<BackgroundJob> get finishedJobs =>
      jobs.where((j) => j.isFinished).toList(growable: false);

  List<BackgroundJob> get completedJobs =>
      jobs.where((j) => j.isCompleted).toList(growable: false);

  List<BackgroundJob> get failedJobs =>
      jobs.where((j) => j.isFailed).toList(growable: false);

  bool get hasActiveJobs => activeJobs.isNotEmpty;
  bool get hasFailedJobs => failedJobs.isNotEmpty;
  bool get hasFinishedJobs => finishedJobs.isNotEmpty;
  bool get isLibraryScanning =>
      activeJobs.any((j) => j.kind == BackgroundJobKind.libraryScan);

  BackgroundJob? get primaryActiveJob =>
      activeJobs.isNotEmpty ? activeJobs.first : null;

  BackgroundJob? get latestError =>
      unacknowledgedErrors.isNotEmpty ? unacknowledgedErrors.first : null;

  double? get aggregateProgress {
    final active = activeJobs;
    if (active.isEmpty) return null;
    if (active.every((j) => j.isIndeterminate)) return null;
    final determinate = active.where((j) => !j.isIndeterminate).toList();
    if (determinate.isEmpty) return null;
    final sum = determinate.fold<double>(
      0.0,
      (acc, j) => acc + j.progressFraction,
    );
    return (sum / determinate.length).clamp(0.0, 1.0);
  }

  BackgroundJobsState copyWith({
    List<BackgroundJob>? jobs,
    List<BackgroundJob>? unacknowledgedErrors,
    bool? isLoading,
  }) {
    return BackgroundJobsState(
      jobs: jobs ?? this.jobs,
      unacknowledgedErrors: unacknowledgedErrors ?? this.unacknowledgedErrors,
      isLoading: isLoading ?? this.isLoading,
    );
  }
}

class BackgroundJobsNotifier extends Notifier<BackgroundJobsState> {
  bool _isDisposed = false;
  final Set<String> _acknowledgedErrorJobIds = {};

  @override
  BackgroundJobsState build() {
    _isDisposed = false;
    ref.onDispose(() {
      _isDisposed = true;
    });
    ref.listen(authProvider, (previous, next) {
      if (previous?.user != next.user && next.user != null) {
        loadJobs();
      }
    });
    Future.microtask(loadJobs);
    return const BackgroundJobsState();
  }

  static List<BackgroundJob> _sortJobs(List<BackgroundJob> list) {
    final copy = List<BackgroundJob>.from(list);
    copy.sort((a, b) {
      if (a.isActive != b.isActive) {
        return a.isActive ? -1 : 1;
      }
      final aTime = a.isActive
          ? (a.startedAt ?? a.updatedAt ?? DateTime.fromMillisecondsSinceEpoch(0))
          : (a.updatedAt ?? a.completedAt ?? DateTime.fromMillisecondsSinceEpoch(0));
      final bTime = b.isActive
          ? (b.startedAt ?? b.updatedAt ?? DateTime.fromMillisecondsSinceEpoch(0))
          : (b.updatedAt ?? b.completedAt ?? DateTime.fromMillisecondsSinceEpoch(0));
      return bTime.compareTo(aTime);
    });
    return copy;
  }

  Future<void> loadJobs() async {
    if (_isDisposed) return;
    state = state.copyWith(isLoading: true);
    try {
      final api = ref.read(apiServiceProvider);
      final list = await api.getBackgroundJobs();
      if (_isDisposed) return;
      final sorted = _sortJobs(list);
      final errors = sorted
          .where((j) => j.isFailed && !_acknowledgedErrorJobIds.contains(j.id))
          .toList(growable: false);
      state = state.copyWith(
        jobs: sorted,
        unacknowledgedErrors: errors,
        isLoading: false,
      );
    } catch (_) {
      if (!_isDisposed) {
        state = state.copyWith(isLoading: false);
      }
    }
  }

  void upsertJob(BackgroundJob job) {
    if (_isDisposed) return;
    final next = List<BackgroundJob>.from(state.jobs);
    final idx = next.indexWhere((j) => j.id == job.id);
    if (idx >= 0) {
      next[idx] = job;
    } else {
      next.add(job);
    }

    var nextErrors = state.unacknowledgedErrors;
    if (job.isFailed && !_acknowledgedErrorJobIds.contains(job.id)) {
      if (!nextErrors.any((e) => e.id == job.id)) {
        nextErrors = [...nextErrors, job];
      }
    }

    state = state.copyWith(
      jobs: _sortJobs(next),
      unacknowledgedErrors: nextErrors,
    );
  }

  void removeJob(String jobId) {
    if (_isDisposed) return;
    state = state.copyWith(
      jobs: state.jobs.where((j) => j.id != jobId).toList(growable: false),
      unacknowledgedErrors: state.unacknowledgedErrors
          .where((j) => j.id != jobId)
          .toList(growable: false),
    );
  }

  void acknowledgeError(String jobId) {
    if (_isDisposed) return;
    _acknowledgedErrorJobIds.add(jobId);
    state = state.copyWith(
      unacknowledgedErrors: state.unacknowledgedErrors
          .where((j) => j.id != jobId)
          .toList(growable: false),
    );
  }

  Future<void> dismissJob(String jobId) async {
    if (_isDisposed) return;
    _acknowledgedErrorJobIds.add(jobId);
    removeJob(jobId);
    try {
      await ref.read(apiServiceProvider).dismissBackgroundJob(jobId);
    } catch (_) {}
  }

  Future<void> clearFinished() async {
    if (_isDisposed) return;
    for (final j in state.finishedJobs) {
      _acknowledgedErrorJobIds.add(j.id);
    }
    state = state.copyWith(
      jobs: state.activeJobs,
      unacknowledgedErrors: const [],
    );
    try {
      await ref.read(apiServiceProvider).clearFinishedBackgroundJobs();
    } catch (_) {}
  }
}

final backgroundJobsProvider =
    NotifierProvider<BackgroundJobsNotifier, BackgroundJobsState>(
  BackgroundJobsNotifier.new,
);
