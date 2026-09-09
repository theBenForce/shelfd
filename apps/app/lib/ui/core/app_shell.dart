import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../features/upload/upload_drop_target.dart';
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
    if (location.startsWith('/settings')) {
      return 2;
    }
    return 0;
  }

  void _onNavTap(BuildContext context, int index) {
    if (index == 0) {
      context.go('/library');
    } else if (index == 1) {
      context.go('/search');
    } else if (index == 2) {
      context.go('/settings');
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final queueState = ref.watch(queueProvider);
    final currentIndex = _calculateSelectedIndex(context);

    Future<void> handleScan() async {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Scanning library...')),
      );
      await ref.read(bookRepositoryProvider).triggerScan();
      if (context.mounted) {
        ref.read(libraryProvider.notifier).loadLibrary(refresh: true);
      }
    }

    return ShelfdAdaptiveScaffold(
      currentIndex: currentIndex,
      onNavTap: (index) => _onNavTap(context, index),
      onRescan: handleScan,
      onUpload: () => pickAndUploadEpub(context, ref),
      isRescanning: queueState.isLoading,
      body: child,
    );
  }
}
