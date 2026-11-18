import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../data/models/book.dart';
import '../../../data/models/yoto.dart';
import '../../core/theme.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';
import 'yoto_icon_picker_dialog.dart';

Future<SendToYotoResult?> showSendToYotoDialog(
  BuildContext context,
  Book book,
) {
  return showDialog<SendToYotoResult>(
    context: context,
    barrierDismissible: false,
    builder: (ctx) => SendToYotoDialog(book: book),
  );
}

class _PlaylistEntryItem {
  final int index;
  final String title;

  const _PlaylistEntryItem({
    required this.index,
    required this.title,
  });
}

class SendToYotoDialog extends ConsumerStatefulWidget {
  final Book book;

  const SendToYotoDialog({
    super.key,
    required this.book,
  });

  @override
  ConsumerState<SendToYotoDialog> createState() => _SendToYotoDialogState();
}

class _SendToYotoDialogState extends ConsumerState<SendToYotoDialog> {
  String _mode = 'existing'; // 'existing' or 'new'
  String _existingAction = 'append'; // 'append' or 'replace'
  String? _selectedCardId;
  late final TextEditingController _newTitleController;
  final TextEditingController _playlistSearchController =
      TextEditingController();
  String _playlistFilter = '';

  bool _includeCover = false;
  bool _useCoverAsIcon = false;
  String _defaultIconUri =
      'yoto:#gTMbacpoeSMYqc9fNLJnxPjylraNG6jIrYEWevyzYbA';
  YotoDisplayIcon? _selectedDefaultIcon;
  final Map<int, YotoDisplayIcon> _chapterCustomIcons = {};
  bool _showChapterIcons = false;

  bool _isSending = false;
  String? _errorMessage;

  @override
  void initState() {
    super.initState();
    _newTitleController = TextEditingController(text: widget.book.title);
  }

  @override
  void dispose() {
    _newTitleController.dispose();
    _playlistSearchController.dispose();
    super.dispose();
  }

  List<_PlaylistEntryItem> _resolveEntries(Book book) {
    final audioFiles = book.files
        .where((f) =>
            f.fileType == 'audiobook' ||
            f.filePath.toLowerCase().endsWith('.m4b') ||
            f.filePath.toLowerCase().endsWith('.m4a') ||
            f.filePath.toLowerCase().endsWith('.mp3'))
        .toList();

    if (audioFiles.length > 1) {
      return List.generate(audioFiles.length, (i) {
        final f = audioFiles[i];
        String label = 'Chapter ${i + 1}';
        if (i < book.audioChapters.length &&
            book.audioChapters[i].title.trim().isNotEmpty) {
          label = book.audioChapters[i].title.trim();
        } else if (f.filePath.isNotEmpty) {
          final parts = f.filePath.split('/');
          label = parts.last;
        }
        return _PlaylistEntryItem(index: i, title: label);
      });
    }

    if (book.audioChapters.isNotEmpty) {
      return List.generate(book.audioChapters.length, (i) {
        final ch = book.audioChapters[i];
        return _PlaylistEntryItem(
          index: i,
          title: ch.title.trim().isNotEmpty ? ch.title.trim() : 'Chapter ${i + 1}',
        );
      });
    }

    return [
      _PlaylistEntryItem(
        index: 0,
        title: book.title.isNotEmpty ? book.title : 'Chapter 1',
      ),
    ];
  }

  Future<void> _pickDefaultIcon() async {
    final picked = await showYotoIconPickerDialog(
      context,
      currentIconUri: _defaultIconUri,
      targetLabel: 'Default icon for all chapters in "${widget.book.title}"',
    );
    if (picked != null && mounted) {
      setState(() {
        _selectedDefaultIcon = picked;
        _defaultIconUri = picked.yotoUri;
        _useCoverAsIcon = false;
      });
    }
  }

  Future<void> _pickChapterIcon(_PlaylistEntryItem entry) async {
    final current = _chapterCustomIcons[entry.index]?.yotoUri ?? _defaultIconUri;
    final picked = await showYotoIconPickerDialog(
      context,
      currentIconUri: current,
      targetLabel: 'Chapter ${entry.index + 1}: ${entry.title}',
    );
    if (picked != null && mounted) {
      setState(() {
        _chapterCustomIcons[entry.index] = picked;
      });
    }
  }

  Future<void> _handleSend() async {
    setState(() {
      _errorMessage = null;
    });

    if (_mode == 'existing' && (_selectedCardId == null || _selectedCardId!.isEmpty)) {
      setState(() {
        _errorMessage = 'Please select a Yoto playlist to add this book to.';
      });
      return;
    }

    final newTitle = _newTitleController.text.trim();
    if (_mode == 'new' && newTitle.isEmpty) {
      setState(() {
        _errorMessage = 'Please enter a title for the new Yoto playlist.';
      });
      return;
    }

    setState(() {
      _isSending = true;
    });

    try {
      final repo = ref.read(yotoRepositoryProvider);
      final chapterIconsPayload = <int, String>{
        for (final entry in _chapterCustomIcons.entries)
          entry.key: entry.value.yotoUri,
      };

      final effectiveMode = _mode == 'existing' ? _existingAction : 'new';
      final request = SendToYotoRequest(
        mode: effectiveMode,
        cardId: _mode == 'existing' ? _selectedCardId : null,
        newPlaylistTitle: _mode == 'new' ? newTitle : null,
        includeCover: _includeCover,
        useCoverAsIcon: _useCoverAsIcon,
        defaultIconId: _defaultIconUri,
        chapterIcons:
            chapterIconsPayload.isNotEmpty ? chapterIconsPayload : null,
      );

      final result = await repo.sendBookToYoto(widget.book.id, request);
      ref.invalidate(yotoPlaylistsProvider);
      if (mounted) {
        Navigator.of(context).pop(result);
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _isSending = false;
          _errorMessage = e.toString().replaceFirst('ApiException', 'Error');
        });
      }
    }
  }

  Widget _buildIconPreviewBadge(
    ShelfdColors colors,
    YotoDisplayIcon? icon, {
    bool isCoverAuto = false,
  }) {
    return Container(
      width: 32,
      height: 32,
      decoration: BoxDecoration(
        color: AppTokens.obsidianBackground,
        borderRadius: BorderRadius.circular(6),
        border: Border.all(color: colors.subtleBorder),
      ),
      alignment: Alignment.center,
      child: isCoverAuto
          ? const Icon(
              Icons.auto_fix_high_rounded,
              size: 16,
              color: AppTokens.amberAccent,
            )
          : (icon != null && icon.url.isNotEmpty
              ? Image.network(
                  icon.url,
                  width: 22,
                  height: 22,
                  fit: BoxFit.contain,
                  filterQuality: FilterQuality.none,
                  errorBuilder: (_, _, _) => const Icon(
                    Icons.auto_stories,
                    size: 16,
                    color: AppTokens.amberAccent,
                  ),
                )
              : const Icon(
                  Icons.auto_stories,
                  size: 16,
                  color: AppTokens.amberAccent,
                )),
    );
  }

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    final yotoAccount = ref.watch(yotoAccountProvider);
    final AsyncValue<List<YotoPlaylist>> playlistsAsync = yotoAccount.isLinked
        ? ref.watch(yotoPlaylistsProvider)
        : const AsyncValue.data([]);
    final entries = _resolveEntries(widget.book);

    return Dialog(
      key: const Key('send_to_yoto_dialog'),
      backgroundColor: colors.surface,
      surfaceTintColor: Colors.transparent,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppTokens.radiusLg),
        side: BorderSide(color: colors.border),
      ),
      child: ConstrainedBox(
        constraints: const BoxConstraints(
          maxWidth: 580,
          maxHeight: 720,
        ),
        child: Padding(
          padding: const EdgeInsets.all(AppTokens.space24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              // Header
              Row(
                children: [
                  Container(
                    padding: const EdgeInsets.all(AppTokens.space8),
                    decoration: BoxDecoration(
                      color: colors.accentContainer,
                      borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                    ),
                    child: Icon(
                      Icons.speaker_rounded,
                      size: 20,
                      color: colors.accentPrimary,
                    ),
                  ),
                  const SizedBox(width: AppTokens.space12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          'Send to Yoto Playlist',
                          style: AppTypography.titleSerif(
                            fontSize: 20,
                            fontWeight: FontWeight.w600,
                            color: colors.ink,
                          ),
                        ),
                        Text(
                          '${widget.book.title} • ${entries.length} ${entries.length == 1 ? 'chapter' : 'chapters'}',
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: AppTypography.captionSans(
                            color: colors.secondaryInk,
                          ),
                        ),
                      ],
                    ),
                  ),
                  IconButton(
                    icon: Icon(Icons.close, color: colors.secondaryInk),
                    onPressed:
                        _isSending ? null : () => Navigator.of(context).pop(),
                  ),
                ],
              ),
              const SizedBox(height: AppTokens.space16),
              Divider(height: 1, color: colors.subtleBorder),
              const SizedBox(height: AppTokens.space16),

              if (!yotoAccount.isLinked && !yotoAccount.isLoading)
                Expanded(
                  child: Center(
                    child: Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        Icon(
                          Icons.link_off_rounded,
                          size: 44,
                          color: colors.mutedInk,
                        ),
                        const SizedBox(height: AppTokens.space12),
                        Text(
                          'Yoto Account Not Linked',
                          style: AppTypography.titleSerif(
                            fontSize: 18,
                            color: colors.ink,
                          ),
                        ),
                        const SizedBox(height: AppTokens.space8),
                        Text(
                          'Connect your Yoto account in Settings to upload audiobooks to your Make Your Own (MYO) playlists.',
                          textAlign: TextAlign.center,
                          style: AppTypography.bodySans(
                            fontSize: 13,
                            color: colors.secondaryInk,
                          ),
                        ),
                        const SizedBox(height: AppTokens.space20),
                        FilledButton.icon(
                          key: const Key('yoto_go_to_settings_button'),
                          style: FilledButton.styleFrom(
                            backgroundColor: colors.accentPrimary,
                            foregroundColor: colors.onAccent,
                          ),
                          onPressed: () {
                            Navigator.of(context).pop();
                            context.go('/settings');
                          },
                          icon: const Icon(Icons.settings_outlined, size: 18),
                          label: const Text('Open Settings to Link Yoto'),
                        ),
                      ],
                    ),
                  ),
                )
              else
                Flexible(
                  child: SingleChildScrollView(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        // Mode Selector: Existing Playlist vs Create New Playlist
                        SegmentedButton<String>(
                          style: ButtonStyle(
                            backgroundColor: WidgetStateProperty.resolveWith(
                              (states) => states.contains(WidgetState.selected)
                                  ? colors.accentContainer
                                  : colors.containerLow,
                            ),
                            foregroundColor: WidgetStateProperty.resolveWith(
                              (states) => states.contains(WidgetState.selected)
                                  ? colors.accentPrimary
                                  : colors.ink,
                            ),
                            side: WidgetStatePropertyAll(
                              BorderSide(color: colors.border),
                            ),
                          ),
                          segments: const [
                            ButtonSegment<String>(
                              value: 'existing',
                              label: Text('Existing Playlist'),
                              icon: Icon(Icons.queue_music_rounded, size: 18),
                            ),
                            ButtonSegment<String>(
                              value: 'new',
                              label: Text('Create New Playlist'),
                              icon: Icon(Icons.playlist_add_rounded, size: 18),
                            ),
                          ],
                          selected: {_mode},
                          onSelectionChanged: _isSending
                              ? null
                              : (selection) {
                                  if (selection.isNotEmpty) {
                                    setState(() {
                                      _mode = selection.first;
                                      _includeCover = _mode == 'new';
                                      _errorMessage = null;
                                    });
                                  }
                                },
                        ),
                        const SizedBox(height: AppTokens.space16),

                        if (_mode == 'existing') ...[
                          SegmentedButton<String>(
                            key: const Key('yoto_existing_action_selector'),
                            style: ButtonStyle(
                              visualDensity: VisualDensity.compact,
                              backgroundColor: WidgetStateProperty.resolveWith(
                                (states) => states.contains(WidgetState.selected)
                                    ? colors.accentContainer
                                    : colors.containerLow,
                              ),
                              foregroundColor: WidgetStateProperty.resolveWith(
                                (states) => states.contains(WidgetState.selected)
                                    ? colors.accentPrimary
                                    : colors.ink,
                              ),
                              side: WidgetStatePropertyAll(
                                BorderSide(color: colors.border),
                              ),
                            ),
                            segments: const [
                              ButtonSegment<String>(
                                value: 'append',
                                label: Text('Append Audio'),
                                icon: Icon(Icons.playlist_add_rounded, size: 16),
                              ),
                              ButtonSegment<String>(
                                value: 'replace',
                                label: Text('Replace Audio'),
                                icon: Icon(Icons.sync_rounded, size: 16),
                              ),
                            ],
                            selected: {_existingAction},
                            onSelectionChanged: _isSending
                                ? null
                                : (selection) {
                                    if (selection.isNotEmpty) {
                                      setState(() {
                                        _existingAction = selection.first;
                                      });
                                    }
                                  },
                          ),
                          const SizedBox(height: AppTokens.space12),
                          TextField(
                            key: const Key('yoto_playlist_search_field'),
                            controller: _playlistSearchController,
                            onChanged: (v) =>
                                setState(() => _playlistFilter = v),
                            style: AppTypography.bodySans(
                              fontSize: 13,
                              color: colors.ink,
                            ),
                            decoration: InputDecoration(
                              hintText: 'Filter your Yoto playlists...',
                              isDense: true,
                              prefixIcon: Icon(
                                Icons.search,
                                size: 18,
                                color: colors.mutedInk,
                              ),
                              border: OutlineInputBorder(
                                borderRadius:
                                    BorderRadius.circular(AppTokens.radiusSm),
                                borderSide: BorderSide(color: colors.border),
                              ),
                              enabledBorder: OutlineInputBorder(
                                borderRadius:
                                    BorderRadius.circular(AppTokens.radiusSm),
                                borderSide: BorderSide(color: colors.border),
                              ),
                            ),
                          ),
                          const SizedBox(height: AppTokens.space8),
                          ConstrainedBox(
                            constraints: const BoxConstraints(maxHeight: 150),
                            child: playlistsAsync.when(
                              loading: () => Padding(
                                padding:
                                    const EdgeInsets.all(AppTokens.space24),
                                child: Center(
                                  child: CircularProgressIndicator(
                                    color: colors.accentPrimary,
                                  ),
                                ),
                              ),
                              error: (err, _) => Padding(
                                padding:
                                    const EdgeInsets.all(AppTokens.space16),
                                child: Text(
                                  'Failed to load playlists: $err',
                                  style: AppTypography.bodySans(
                                    fontSize: 13,
                                    color: colors.errorText,
                                  ),
                                ),
                              ),
                              data: (playlists) {
                                final q = _playlistFilter.trim().toLowerCase();
                                final filtered = q.isEmpty
                                    ? playlists
                                    : playlists
                                        .where((p) =>
                                            p.title.toLowerCase().contains(q))
                                        .toList();

                                if (filtered.isEmpty) {
                                  return Container(
                                    padding:
                                        const EdgeInsets.all(AppTokens.space16),
                                    decoration: BoxDecoration(
                                      color: colors.containerLow,
                                      borderRadius: BorderRadius.circular(
                                          AppTokens.radiusSm),
                                      border: Border.all(
                                          color: colors.subtleBorder),
                                    ),
                                    child: Column(
                                      mainAxisSize: MainAxisSize.min,
                                      children: [
                                        Text(
                                          playlists.isEmpty
                                              ? 'No existing Yoto playlists found.'
                                              : 'No playlists match "$_playlistFilter".',
                                          style: AppTypography.bodySans(
                                            fontSize: 13,
                                            color: colors.secondaryInk,
                                          ),
                                        ),
                                        if (playlists.isEmpty) ...[
                                          const SizedBox(
                                              height: AppTokens.space8),
                                          TextButton.icon(
                                            onPressed: () {
                                              setState(() {
                                                _mode = 'new';
                                                _includeCover = true;
                                              });
                                            },
                                            icon: const Icon(
                                                Icons.add_rounded,
                                                size: 16),
                                            label: const Text(
                                                'Create New Playlist Instead'),
                                          ),
                                        ],
                                      ],
                                    ),
                                  );
                                }

                                // Auto-select first playlist if none selected yet
                                if (_selectedCardId == null &&
                                    filtered.isNotEmpty) {
                                  _selectedCardId = filtered.first.cardId;
                                }

                                return ListView.separated(
                                  shrinkWrap: true,
                                  itemCount: filtered.length,
                                  separatorBuilder: (_, _) =>
                                      const SizedBox(height: AppTokens.space4),
                                  itemBuilder: (context, idx) {
                                    final pl = filtered[idx];
                                    final selected =
                                        _selectedCardId == pl.cardId;
                                    return InkWell(
                                      key: Key(
                                          'yoto_playlist_option_${pl.cardId}'),
                                      borderRadius: BorderRadius.circular(
                                          AppTokens.radiusSm),
                                      onTap: _isSending
                                          ? null
                                          : () {
                                              setState(() {
                                                _selectedCardId = pl.cardId;
                                              });
                                            },
                                      child: Container(
                                        padding: const EdgeInsets.symmetric(
                                          horizontal: AppTokens.space12,
                                          vertical: AppTokens.space8,
                                        ),
                                        decoration: BoxDecoration(
                                          color: selected
                                              ? colors.accentContainer
                                              : colors.containerLow,
                                          borderRadius: BorderRadius.circular(
                                              AppTokens.radiusSm),
                                          border: Border.all(
                                            color: selected
                                                ? colors.accentPrimary
                                                : colors.subtleBorder,
                                          ),
                                        ),
                                        child: Row(
                                          children: [
                                            Icon(
                                              selected
                                                  ? Icons.radio_button_checked
                                                  : Icons.radio_button_off,
                                              size: 18,
                                              color: selected
                                                  ? colors.accentPrimary
                                                  : colors.mutedInk,
                                            ),
                                            const SizedBox(
                                                width: AppTokens.space12),
                                            Expanded(
                                              child: Column(
                                                crossAxisAlignment:
                                                    CrossAxisAlignment.start,
                                                children: [
                                                  Text(
                                                    pl.title,
                                                    maxLines: 1,
                                                    overflow:
                                                        TextOverflow.ellipsis,
                                                    style:
                                                        AppTypography.bodySans(
                                                      fontSize: 14,
                                                      fontWeight: selected
                                                          ? FontWeight.w600
                                                          : FontWeight.w500,
                                                      color: colors.ink,
                                                    ),
                                                  ),
                                                  Text(
                                                    '${pl.chapterCount} ${pl.chapterCount == 1 ? 'chapter' : 'chapters'}',
                                                    style: AppTypography
                                                        .captionSans(
                                                      fontSize: 11,
                                                      color:
                                                          colors.secondaryInk,
                                                    ),
                                                  ),
                                                ],
                                              ),
                                            ),
                                          ],
                                        ),
                                      ),
                                    );
                                  },
                                );
                              },
                            ),
                          ),
                        ] else ...[
                          TextField(
                            key: const Key('yoto_new_playlist_title_field'),
                            controller: _newTitleController,
                            enabled: !_isSending,
                            style: AppTypography.bodySans(
                              fontSize: 14,
                              color: colors.ink,
                            ),
                            decoration: InputDecoration(
                              labelText: 'New Playlist Title',
                              border: OutlineInputBorder(
                                borderRadius:
                                    BorderRadius.circular(AppTokens.radiusSm),
                                borderSide: BorderSide(color: colors.border),
                              ),
                            ),
                          ),
                        ],
                        const SizedBox(height: AppTokens.space8),
                        Material(
                          type: MaterialType.transparency,
                          child: CheckboxListTile(
                            key: const Key('yoto_include_cover_checkbox'),
                            contentPadding: EdgeInsets.zero,
                            dense: true,
                            controlAffinity: ListTileControlAffinity.leading,
                            title: Text(
                              _mode == 'existing'
                                  ? 'Update playlist cover with this book\'s cover'
                                  : 'Upload book cover art as Yoto playlist cover',
                              style: AppTypography.bodySans(
                                fontSize: 13,
                                color: colors.ink,
                              ),
                            ),
                            value: _includeCover,
                            activeColor: colors.accentPrimary,
                            onChanged: _isSending
                                ? null
                                : (val) {
                                    setState(() {
                                      _includeCover = val ?? false;
                                    });
                                  },
                          ),
                        ),

                        const SizedBox(height: AppTokens.space16),
                        Divider(height: 1, color: colors.subtleBorder),
                        const SizedBox(height: AppTokens.space12),

                        // 16x16 Display Icons Configuration Section
                        Text(
                          'Playlist Entry Icons (16×16 Pixel Display)',
                          style: AppTypography.bodySans(
                            fontSize: 14,
                            fontWeight: FontWeight.w600,
                            color: colors.ink,
                          ),
                        ),
                        const SizedBox(height: AppTokens.space4),
                        Text(
                          'Configure the 16×16 pixel icons shown on the Yoto Player face for each chapter.',
                          style: AppTypography.captionSans(
                            color: colors.secondaryInk,
                          ),
                        ),
                        const SizedBox(height: AppTokens.space12),

                        Container(
                          padding: const EdgeInsets.all(AppTokens.space12),
                          decoration: BoxDecoration(
                            color: colors.containerLow,
                            borderRadius:
                                BorderRadius.circular(AppTokens.radiusSm),
                            border: Border.all(color: colors.subtleBorder),
                          ),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.stretch,
                            children: [
                              Row(
                                children: [
                                  _buildIconPreviewBadge(
                                    colors,
                                    _selectedDefaultIcon,
                                    isCoverAuto: _useCoverAsIcon,
                                  ),
                                  const SizedBox(width: AppTokens.space12),
                                  Expanded(
                                    child: Column(
                                      crossAxisAlignment:
                                          CrossAxisAlignment.start,
                                      children: [
                                        Text(
                                          'Default Chapter Icon',
                                          style: AppTypography.bodySans(
                                            fontSize: 13,
                                            fontWeight: FontWeight.w600,
                                            color: colors.ink,
                                          ),
                                        ),
                                        Text(
                                          _useCoverAsIcon
                                              ? 'Auto-converted 16×16 from book cover'
                                              : (_selectedDefaultIcon != null &&
                                                      _selectedDefaultIcon!
                                                          .title.isNotEmpty
                                                  ? _selectedDefaultIcon!.title
                                                  : 'Default Yoto Book Icon'),
                                          style: AppTypography.captionSans(
                                            fontSize: 12,
                                            color: colors.secondaryInk,
                                          ),
                                        ),
                                      ],
                                    ),
                                  ),
                                  OutlinedButton.icon(
                                    key: const Key('yoto_change_default_icon_button'),
                                    style: OutlinedButton.styleFrom(
                                      foregroundColor: colors.ink,
                                      side: BorderSide(color: colors.border),
                                      visualDensity: VisualDensity.compact,
                                    ),
                                    onPressed:
                                        _isSending ? null : _pickDefaultIcon,
                                    icon: const Icon(
                                        Icons.grid_view_rounded,
                                        size: 15),
                                    label: const Text('Choose Icon'),
                                  ),
                                ],
                              ),
                              const SizedBox(height: AppTokens.space8),
                              Material(
                                type: MaterialType.transparency,
                                child: SwitchListTile.adaptive(
                                  key: const Key('yoto_use_cover_as_icon_switch'),
                                  contentPadding: EdgeInsets.zero,
                                  dense: true,
                                  title: Text(
                                    'Auto-convert book cover to 16×16 chapter icon',
                                    style: AppTypography.bodySans(
                                      fontSize: 13,
                                      color: colors.ink,
                                    ),
                                  ),
                                  value: _useCoverAsIcon,
                                  activeTrackColor: colors.accentPrimary,
                                  onChanged: _isSending
                                      ? null
                                      : (val) {
                                          setState(() {
                                            _useCoverAsIcon = val;
                                          });
                                        },
                                ),
                              ),
                            ],
                          ),
                        ),
                        const SizedBox(height: AppTokens.space8),

                        // Per-Chapter Icon Overrides
                        InkWell(
                          key: const Key('yoto_customize_chapter_icons_tile'),
                          borderRadius:
                              BorderRadius.circular(AppTokens.radiusSm),
                          onTap: () {
                            setState(() {
                              _showChapterIcons = !_showChapterIcons;
                            });
                          },
                          child: Padding(
                            padding: const EdgeInsets.symmetric(
                              vertical: AppTokens.space8,
                              horizontal: AppTokens.space4,
                            ),
                            child: Row(
                              children: [
                                Icon(
                                  _showChapterIcons
                                      ? Icons.expand_less_rounded
                                      : Icons.expand_more_rounded,
                                  size: 20,
                                  color: colors.accentPrimary,
                                ),
                                const SizedBox(width: AppTokens.space8),
                                Flexible(
                                  child: Text(
                                    'Customize Icons per Playlist Entry (${entries.length})',
                                    maxLines: 1,
                                    overflow: TextOverflow.ellipsis,
                                    style: AppTypography.bodySans(
                                      fontSize: 13,
                                      fontWeight: FontWeight.w600,
                                      color: colors.accentPrimary,
                                    ),
                                  ),
                                ),
                                if (_chapterCustomIcons.isNotEmpty) ...[
                                  const SizedBox(width: AppTokens.space8),
                                  Container(
                                    padding: const EdgeInsets.symmetric(
                                      horizontal: 6,
                                      vertical: 2,
                                    ),
                                    decoration: BoxDecoration(
                                      color: colors.accentContainer,
                                      borderRadius: BorderRadius.circular(
                                          AppTokens.radiusPill),
                                    ),
                                    child: Text(
                                      '${_chapterCustomIcons.length} custom',
                                      style: AppTypography.captionSans(
                                        fontSize: 11,
                                        color: colors.accentPrimary,
                                      ),
                                    ),
                                  ),
                                ],
                              ],
                            ),
                          ),
                        ),

                        if (_showChapterIcons)
                          Container(
                            constraints: const BoxConstraints(maxHeight: 200),
                            decoration: BoxDecoration(
                              color: colors.containerLow,
                              borderRadius:
                                  BorderRadius.circular(AppTokens.radiusSm),
                              border: Border.all(color: colors.subtleBorder),
                            ),
                            child: ListView.separated(
                              shrinkWrap: true,
                              padding: const EdgeInsets.all(AppTokens.space8),
                              itemCount: entries.length,
                              separatorBuilder: (_, _) => Divider(
                                height: 1,
                                color: colors.subtleBorder,
                              ),
                              itemBuilder: (context, i) {
                                final entry = entries[i];
                                final customIcon =
                                    _chapterCustomIcons[entry.index];
                                return Padding(
                                  padding: const EdgeInsets.symmetric(
                                    vertical: AppTokens.space4,
                                  ),
                                  child: Row(
                                    children: [
                                      InkWell(
                                        key: Key(
                                            'yoto_chapter_icon_button_${entry.index}'),
                                        onTap: _isSending
                                            ? null
                                            : () => _pickChapterIcon(entry),
                                        borderRadius: BorderRadius.circular(6),
                                        child: _buildIconPreviewBadge(
                                          colors,
                                          customIcon ?? _selectedDefaultIcon,
                                          isCoverAuto: customIcon == null &&
                                              _useCoverAsIcon,
                                        ),
                                      ),
                                      const SizedBox(width: AppTokens.space12),
                                      Expanded(
                                        child: Column(
                                          crossAxisAlignment:
                                              CrossAxisAlignment.start,
                                          children: [
                                            Text(
                                              '${entry.index + 1}. ${entry.title}',
                                              maxLines: 1,
                                              overflow: TextOverflow.ellipsis,
                                              style: AppTypography.bodySans(
                                                fontSize: 13,
                                                color: colors.ink,
                                              ),
                                            ),
                                            if (customIcon != null)
                                              Text(
                                                customIcon.title.isNotEmpty
                                                    ? 'Custom: ${customIcon.title}'
                                                    : 'Custom 16×16 icon selected',
                                                style:
                                                    AppTypography.captionSans(
                                                  fontSize: 11,
                                                  color: colors.accentPrimary,
                                                ),
                                              ),
                                          ],
                                        ),
                                      ),
                                      if (customIcon != null)
                                        IconButton(
                                          tooltip: 'Reset to default icon',
                                          icon: Icon(
                                            Icons.restart_alt_rounded,
                                            size: 16,
                                            color: colors.mutedInk,
                                          ),
                                          onPressed: _isSending
                                              ? null
                                              : () {
                                                  setState(() {
                                                    _chapterCustomIcons
                                                        .remove(entry.index);
                                                  });
                                                },
                                        ),
                                      TextButton(
                                        onPressed: _isSending
                                            ? null
                                            : () => _pickChapterIcon(entry),
                                        child: const Text('Set Icon'),
                                      ),
                                    ],
                                  ),
                                );
                              },
                            ),
                          ),
                      ],
                    ),
                  ),
                ),

              if (_errorMessage != null) ...[
                const SizedBox(height: AppTokens.space12),
                Container(
                  padding: const EdgeInsets.all(AppTokens.space12),
                  decoration: BoxDecoration(
                    color: colors.errorBg,
                    borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                    border: Border.all(color: colors.errorBorder),
                  ),
                  child: Text(
                    _errorMessage!,
                    style: AppTypography.bodySans(
                      fontSize: 13,
                      color: colors.errorText,
                    ),
                  ),
                ),
              ],

              if (_isSending) ...[
                const SizedBox(height: AppTokens.space12),
                Row(
                  children: [
                    SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(
                        strokeWidth: 2,
                        color: colors.accentPrimary,
                      ),
                    ),
                    const SizedBox(width: AppTokens.space12),
                    Expanded(
                      child: Text(
                        'Uploading & transcoding audio tracks on Yoto...',
                        style: AppTypography.captionSans(
                          color: colors.secondaryInk,
                        ),
                      ),
                    ),
                  ],
                ),
              ],

              const SizedBox(height: AppTokens.space16),
              Row(
                mainAxisAlignment: MainAxisAlignment.end,
                children: [
                  TextButton(
                    onPressed:
                        _isSending ? null : () => Navigator.of(context).pop(),
                    child: Text(
                      'Cancel',
                      style: AppTypography.bodySans(
                        fontSize: 14,
                        color: colors.secondaryInk,
                      ),
                    ),
                  ),
                  if (yotoAccount.isLinked) ...[
                    const SizedBox(width: AppTokens.space8),
                    FilledButton.icon(
                      key: const Key('confirm_send_to_yoto_button'),
                      style: FilledButton.styleFrom(
                        backgroundColor: colors.accentPrimary,
                        foregroundColor: colors.onAccent,
                      ),
                      onPressed: _isSending ? null : _handleSend,
                      icon: const Icon(Icons.send_rounded, size: 16),
                      label: Text(
                        _isSending
                            ? 'Sending...'
                            : (_mode == 'existing'
                                ? (_existingAction == 'append'
                                    ? 'Append to Playlist'
                                    : 'Replace Playlist')
                                : 'Send to Yoto'),
                      ),
                    ),
                  ],
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}
