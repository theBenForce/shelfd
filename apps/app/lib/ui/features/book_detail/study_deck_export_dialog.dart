import 'package:flutter/material.dart';
import '../../../data/models/book.dart';
import '../../core/theme.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';

enum StudyDeckFormat {
  apkg(
    apiValue: 'apkg',
    label: 'Anki .apkg',
    description: 'Native Anki package with SQLite deck database',
    fileSuffix: '-study-deck.apkg',
    mimeType: 'application/octet-stream',
  ),
  csv(
    apiValue: 'csv',
    label: 'Anki .csv',
    description: 'UTF-8 CSV flashcards for Anki or spreadsheets',
    fileSuffix: '-study-deck.csv',
    mimeType: 'text/csv; charset=utf-8',
  ),
  markdown(
    apiValue: 'markdown',
    label: 'Markdown Vault .zip',
    description: 'Obsidian-ready ZIP with concept notes and backlinks',
    fileSuffix: '-study-vault.zip',
    mimeType: 'application/zip',
  );

  final String apiValue;
  final String label;
  final String description;
  final String fileSuffix;
  final String mimeType;

  const StudyDeckFormat({
    required this.apiValue,
    required this.label,
    required this.description,
    required this.fileSuffix,
    required this.mimeType,
  });
}

class StudyDeckExportSelection {
  final StudyDeckFormat format;
  final List<int> chapterIndices;

  const StudyDeckExportSelection({
    required this.format,
    required this.chapterIndices,
  });
}

Future<StudyDeckExportSelection?> showStudyDeckExportDialog(
  BuildContext context,
  Book book,
) {
  return showDialog<StudyDeckExportSelection>(
    context: context,
    builder: (ctx) => StudyDeckExportDialog(book: book),
  );
}

class StudyDeckExportDialog extends StatefulWidget {
  final Book book;

  const StudyDeckExportDialog({
    super.key,
    required this.book,
  });

  @override
  State<StudyDeckExportDialog> createState() => _StudyDeckExportDialogState();
}

class _StudyDeckExportDialogState extends State<StudyDeckExportDialog> {
  StudyDeckFormat _selectedFormat = StudyDeckFormat.apkg;
  late final List<SpineItem> _chapters;
  late Set<int> _selectedChapterIndices;

  @override
  void initState() {
    super.initState();
    _chapters = widget.book.isFixedLayout && widget.book.namedSpine.isNotEmpty
        ? widget.book.namedSpine
        : widget.book.spine;
    _selectedChapterIndices = _chapters.map((c) => c.chapterIndex).toSet();
  }

  void _selectAll() {
    setState(() {
      _selectedChapterIndices = _chapters.map((c) => c.chapterIndex).toSet();
    });
  }

  void _clearAll() {
    setState(() {
      _selectedChapterIndices = <int>{};
    });
  }

  void _toggleChapter(int chapterIndex, bool selected) {
    setState(() {
      if (selected) {
        _selectedChapterIndices.add(chapterIndex);
      } else {
        _selectedChapterIndices.remove(chapterIndex);
      }
    });
  }

  void _submit() {
    if (_selectedChapterIndices.isEmpty) return;
    final sorted = _selectedChapterIndices.toList()..sort();
    Navigator.of(context).pop(
      StudyDeckExportSelection(
        format: _selectedFormat,
        chapterIndices: sorted,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    final canExport = _selectedChapterIndices.isNotEmpty;

    return Dialog(
      key: const Key('study_deck_export_dialog'),
      backgroundColor: colors.surface,
      surfaceTintColor: Colors.transparent,
      insetPadding: const EdgeInsets.symmetric(
        horizontal: AppTokens.space16,
        vertical: AppTokens.space24,
      ),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        side: BorderSide(color: colors.border),
      ),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 520, maxHeight: 680),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(
                AppTokens.space20,
                AppTokens.space20,
                AppTokens.space20,
                AppTokens.space12,
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Export Study Deck',
                    style: AppTypography.titleSerif(
                      fontSize: 20,
                      fontWeight: FontWeight.w700,
                      color: colors.ink,
                    ),
                  ),
                  const SizedBox(height: AppTokens.space4),
                  Text(
                    'Choose an export format and chapters for ${widget.book.title}.',
                    style: AppTypography.bodySans(
                      fontSize: 13,
                      color: colors.secondaryInk,
                    ),
                  ),
                ],
              ),
            ),
            Divider(height: 1, color: colors.subtleBorder),
            Flexible(
              child: SingleChildScrollView(
                padding: const EdgeInsets.all(AppTokens.space20),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'Format',
                      style: AppTypography.captionSans(
                        fontSize: 12,
                        fontWeight: FontWeight.w600,
                        color: colors.secondaryInk,
                      ),
                    ),
                    const SizedBox(height: AppTokens.space8),
                    RadioGroup<StudyDeckFormat>(
                      groupValue: _selectedFormat,
                      onChanged: (value) {
                        if (value != null) {
                          setState(() => _selectedFormat = value);
                        }
                      },
                      child: Column(
                        children: [
                          for (final format in StudyDeckFormat.values)
                            RadioListTile<StudyDeckFormat>(
                              key: Key('study_deck_format_${format.apiValue}'),
                              value: format,
                              dense: true,
                              contentPadding: const EdgeInsets.symmetric(
                                horizontal: AppTokens.space8,
                              ),
                              activeColor: colors.accentPrimary,
                              shape: RoundedRectangleBorder(
                                borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                              ),
                              title: Text(
                                format.label,
                                style: AppTypography.bodySans(
                                  fontSize: 14,
                                  fontWeight: FontWeight.w600,
                                  color: colors.ink,
                                ),
                              ),
                              subtitle: Text(
                                format.description,
                                style: AppTypography.captionSans(
                                  fontSize: 12,
                                  color: colors.secondaryInk,
                                ),
                              ),
                            ),
                        ],
                      ),
                    ),
                    const SizedBox(height: AppTokens.space16),
                    Row(
                      children: [
                        Expanded(
                          child: Text(
                            'Chapters (${_selectedChapterIndices.length}/${_chapters.length})',
                            style: AppTypography.captionSans(
                              fontSize: 12,
                              fontWeight: FontWeight.w600,
                              color: colors.secondaryInk,
                            ),
                          ),
                        ),
                        TextButton(
                          key: const Key('study_deck_select_all_button'),
                          onPressed: _chapters.isEmpty ? null : _selectAll,
                          style: TextButton.styleFrom(
                            visualDensity: VisualDensity.compact,
                          ),
                          child: const Text('Select All'),
                        ),
                        TextButton(
                          key: const Key('study_deck_clear_all_button'),
                          onPressed: _selectedChapterIndices.isEmpty ? null : _clearAll,
                          style: TextButton.styleFrom(
                            visualDensity: VisualDensity.compact,
                          ),
                          child: const Text('Clear All'),
                        ),
                      ],
                    ),
                    const SizedBox(height: AppTokens.space4),
                    if (_chapters.isEmpty)
                      Padding(
                        padding: const EdgeInsets.symmetric(vertical: AppTokens.space12),
                        child: Text(
                          'No chapters available to export.',
                          style: AppTypography.bodySans(
                            fontSize: 13,
                            color: colors.mutedInk,
                          ),
                        ),
                      )
                    else
                      Material(
                        color: colors.containerLow,
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                          side: BorderSide(color: colors.subtleBorder),
                        ),
                        clipBehavior: Clip.antiAlias,
                        child: Column(
                          children: [
                            for (final chapter in _chapters)
                              CheckboxListTile(
                                key: Key('study_deck_chapter_${chapter.chapterIndex}'),
                                value: _selectedChapterIndices.contains(chapter.chapterIndex),
                                onChanged: (checked) =>
                                    _toggleChapter(chapter.chapterIndex, checked ?? false),
                                dense: true,
                                controlAffinity: ListTileControlAffinity.leading,
                                activeColor: colors.accentPrimary,
                                title: Text(
                                  chapter.title.isNotEmpty
                                      ? chapter.title
                                      : 'Chapter ${chapter.chapterIndex}',
                                  maxLines: 2,
                                  overflow: TextOverflow.ellipsis,
                                  style: AppTypography.bodySans(
                                    fontSize: 13,
                                    color: colors.ink,
                                  ),
                                ),
                              ),
                          ],
                        ),
                      ),
                  ],
                ),
              ),
            ),
            Divider(height: 1, color: colors.subtleBorder),
            Padding(
              padding: const EdgeInsets.symmetric(
                horizontal: AppTokens.space20,
                vertical: AppTokens.space12,
              ),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.end,
                children: [
                  TextButton(
                    key: const Key('study_deck_cancel_button'),
                    onPressed: () => Navigator.of(context).pop(),
                    child: Text(
                      'Cancel',
                      style: AppTypography.bodySans(
                        fontSize: 14,
                        color: colors.secondaryInk,
                      ),
                    ),
                  ),
                  const SizedBox(width: AppTokens.space8),
                  FilledButton(
                    key: const Key('study_deck_export_button'),
                    style: FilledButton.styleFrom(
                      backgroundColor: colors.accentPrimary,
                      foregroundColor: colors.onAccent,
                    ),
                    onPressed: canExport ? _submit : null,
                    child: const Text('Export'),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
