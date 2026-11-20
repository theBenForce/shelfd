import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../state/providers.dart';
import 'shared_layout.dart';

/// Persistent application navigation shell wrapping primary routes in [ShelfdAdaptiveScaffold].
class AppShell extends ConsumerWidget {
  final Widget child;

  const AppShell({super.key, required this.child});

  int _calculateSelectedIndex(BuildContext context) {
    final location = GoRouterState.of(context).uri.path;
    if (location.startsWith('/search')) {
      return 1;
    }
    if (location.startsWith('/uploads') ||
        location.startsWith('/plugins/catalog')) {
      return 2;
    }
    if (location.startsWith('/utilities')) {
      return 3;
    }
    if (location.startsWith('/settings')) {
      return 4;
    }
    return 0;
  }

  void _onNavTap(BuildContext context, int index) {
    switch (index) {
      case 0:
        context.go('/books');
      case 1:
        context.go('/search');
      case 2:
        context.go('/uploads');
      case 3:
        context.go('/utilities');
      case 4:
        context.go('/settings');
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // Keep SSE stream connected for real-time queue & background job updates.
    ref.watch(queueProvider);
    final bgJobsState = ref.watch(backgroundJobsProvider);
    final currentPath = GoRouterState.of(context).uri.path;
    final currentIndex = _calculateSelectedIndex(context);

    Future<void> handleScan() async {
      final job = await ref.read(bookRepositoryProvider).triggerScan();
      if (job != null) {
        ref.read(backgroundJobsProvider.notifier).upsertJob(job);
      }
    }

    return ShelfdAdaptiveScaffold(
      currentIndex: currentIndex,
      onNavTap: (index) => _onNavTap(context, index),
      currentPath: currentPath,
      onNavigate: (path) => context.go(path),
      onRescan: handleScan,
      onUpload: () => context.go('/uploads'),
      isRescanning: bgJobsState.isLibraryScanning,
      body: child,
    );
  }
}
