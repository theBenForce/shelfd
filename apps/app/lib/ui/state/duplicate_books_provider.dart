import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../data/models/book.dart';
import '../../data/models/duplicate_group.dart';
import 'providers.dart';

class DuplicateBooksState {
  final bool isScanning;
  final bool isMerging;
  final String? mergingGroupId;
  final DuplicateScanResult? result;
  final String? errorMessage;
  final String? successMessage;
  final Map<String, String> selectedPrimaryBooks;

  const DuplicateBooksState({
    this.isScanning = false,
    this.isMerging = false,
    this.mergingGroupId,
    this.result,
    this.errorMessage,
    this.successMessage,
    this.selectedPrimaryBooks = const {},
  });

  DuplicateBooksState copyWith({
    bool? isScanning,
    bool? isMerging,
    String? mergingGroupId,
    DuplicateScanResult? result,
    String? errorMessage,
    String? successMessage,
    Map<String, String>? selectedPrimaryBooks,
  }) {
    return DuplicateBooksState(
      isScanning: isScanning ?? this.isScanning,
      isMerging: isMerging ?? this.isMerging,
      mergingGroupId: mergingGroupId,
      result: result ?? this.result,
      errorMessage: errorMessage,
      successMessage: successMessage,
      selectedPrimaryBooks: selectedPrimaryBooks ?? this.selectedPrimaryBooks,
    );
  }
}

class DuplicateBooksNotifier extends Notifier<DuplicateBooksState> {
  @override
  DuplicateBooksState build() {
    return const DuplicateBooksState();
  }

  void selectPrimaryBook(String groupId, String bookId) {
    final updated = Map<String, String>.from(state.selectedPrimaryBooks);
    updated[groupId] = bookId;
    state = state.copyWith(selectedPrimaryBooks: updated);
  }

  void clearMessages() {
    state = state.copyWith(errorMessage: null, successMessage: null);
  }

  Future<void> scanForDuplicates() async {
    state = state.copyWith(
      isScanning: true,
      errorMessage: null,
      successMessage: null,
    );
    try {
      final bookRepo = ref.read(bookRepositoryProvider);
      final res = await bookRepo.getDuplicateBooks();
      final primaryMap = <String, String>{};
      for (final group in res.groups) {
        if (group.books.isNotEmpty) {
          primaryMap[group.id] = group.books.first.id;
        }
      }
      state = state.copyWith(
        isScanning: false,
        result: res,
        selectedPrimaryBooks: primaryMap,
      );
    } catch (e) {
      state = state.copyWith(
        isScanning: false,
        errorMessage: 'Failed to scan for duplicate books: $e',
      );
    }
  }

  Future<Book?> mergeGroup({
    required DuplicateGroup group,
    required String primaryBookId,
    MergeOptions? options,
  }) async {
    final duplicateIds = group.books
        .map((b) => b.id)
        .where((id) => id != primaryBookId)
        .toList();

    if (duplicateIds.isEmpty) return null;

    state = state.copyWith(
      isMerging: true,
      mergingGroupId: group.id,
      errorMessage: null,
      successMessage: null,
    );

    try {
      final bookRepo = ref.read(bookRepositoryProvider);
      final mergedBook = await bookRepo.mergeBooks(
        primaryBookId: primaryBookId,
        duplicateBookIds: duplicateIds,
        options: options,
      );

      // Refresh global library state
      ref.read(libraryProvider.notifier).loadLibrary(refresh: true);

      // Remove merged group from local state
      final currentGroups = state.result?.groups ?? [];
      final remainingGroups =
          currentGroups.where((g) => g.id != group.id).toList();
      final updatedTotalBooks =
          (state.result?.totalDuplicateBooks ?? 0) - group.books.length;

      final updatedResult = DuplicateScanResult(
        groups: remainingGroups,
        totalGroups: remainingGroups.length,
        totalDuplicateBooks: updatedTotalBooks > 0 ? updatedTotalBooks : 0,
      );

      final updatedPrimary =
          Map<String, String>.from(state.selectedPrimaryBooks);
      updatedPrimary.remove(group.id);

      state = state.copyWith(
        isMerging: false,
        mergingGroupId: null,
        result: updatedResult,
        selectedPrimaryBooks: updatedPrimary,
        successMessage:
            'Successfully merged "${mergedBook.title}" (${duplicateIds.length} duplicate copy removed)',
      );

      return mergedBook;
    } catch (e) {
      state = state.copyWith(
        isMerging: false,
        mergingGroupId: null,
        errorMessage: 'Failed to merge books: $e',
      );
      return null;
    }
  }
}

final duplicateBooksProvider =
    NotifierProvider<DuplicateBooksNotifier, DuplicateBooksState>(
        DuplicateBooksNotifier.new);
