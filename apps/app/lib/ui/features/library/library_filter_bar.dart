import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../core/tokens.dart';
import '../../core/typography.dart';
import '../../state/providers.dart';
import 'library_query_parser.dart';

/// Autocomplete suggestion item representing an author, genre, or topic.
class AutocompleteSuggestion {
  final String name;
  final String tag;
  final IconData icon;
  final int? count;

  const AutocompleteSuggestion({
    required this.name,
    required this.tag,
    required this.icon,
    this.count,
  });
}

/// Search and filter bar for the library view.
/// Supports free-form book title filtering and tag syntax like:
/// `Author:"Name"`, `Genre:"Name"`, `Topic:"Name"`.
/// Shows keyboard-navigable autocomplete suggestions when typing `author:`, `genre:`, or `topic:`.
class LibraryFilterBar extends ConsumerStatefulWidget {
  final String? hintText;
  final VoidCallback? onSubmitted;

  const LibraryFilterBar({
    super.key,
    this.hintText,
    this.onSubmitted,
  });

  @override
  ConsumerState<LibraryFilterBar> createState() => _LibraryFilterBarState();
}

class _LibraryFilterBarState extends ConsumerState<LibraryFilterBar> {
  late final TextEditingController _controller;
  late final FocusNode _focusNode;
  final LayerLink _layerLink = LayerLink();

  OverlayEntry? _overlayEntry;
  List<AutocompleteSuggestion> _suggestions = [];
  int _highlightedIndex = 0;
  ({String tag, String prefix, int tokenStart, int tokenEnd})? _currentPrefix;

  @override
  void initState() {
    super.initState();
    final initialQuery = ref.read(libraryProvider).searchQuery;
    _controller = TextEditingController(text: initialQuery);
    _focusNode = FocusNode();

    _controller.addListener(_onTextChanged);
    _focusNode.addListener(_onFocusChanged);
    _focusNode.onKeyEvent = _handleKeyEvent;
  }

  @override
  void dispose() {
    _hideOverlay();
    _controller.removeListener(_onTextChanged);
    _focusNode.removeListener(_onFocusChanged);
    _controller.dispose();
    _focusNode.dispose();
    super.dispose();
  }

  void _onFocusChanged() {
    if (!_focusNode.hasFocus) {
      Future.delayed(const Duration(milliseconds: 150), () {
        if (mounted && !_focusNode.hasFocus) {
          _hideOverlay();
        }
      });
    } else {
      _checkAutocomplete();
    }
  }

  void _onTextChanged() {
    setState(() {});
    final text = _controller.text;
    ref.read(libraryProvider.notifier).setSearchQuery(text);
    _checkAutocomplete();
  }

  KeyEventResult _handleKeyEvent(FocusNode node, KeyEvent event) {
    if (event is! KeyDownEvent) return KeyEventResult.ignored;

    if (_overlayEntry != null && _suggestions.isNotEmpty) {
      if (event.logicalKey == LogicalKeyboardKey.arrowDown) {
        setState(() {
          _highlightedIndex = (_highlightedIndex + 1) % _suggestions.length;
        });
        _overlayEntry?.markNeedsBuild();
        return KeyEventResult.handled;
      } else if (event.logicalKey == LogicalKeyboardKey.arrowUp) {
        setState(() {
          _highlightedIndex = (_highlightedIndex - 1 + _suggestions.length) % _suggestions.length;
        });
        _overlayEntry?.markNeedsBuild();
        return KeyEventResult.handled;
      } else if (event.logicalKey == LogicalKeyboardKey.enter ||
          event.logicalKey == LogicalKeyboardKey.numpadEnter) {
        if (_highlightedIndex >= 0 && _highlightedIndex < _suggestions.length) {
          _selectSuggestion(_suggestions[_highlightedIndex]);
          return KeyEventResult.handled;
        }
      } else if (event.logicalKey == LogicalKeyboardKey.escape) {
        _hideOverlay();
        return KeyEventResult.handled;
      }
    }

    if (event.logicalKey == LogicalKeyboardKey.enter ||
        event.logicalKey == LogicalKeyboardKey.numpadEnter) {
      widget.onSubmitted?.call();
      return KeyEventResult.handled;
    }

    return KeyEventResult.ignored;
  }

  void _checkAutocomplete() {
    if (!_focusNode.hasFocus) {
      _hideOverlay();
      return;
    }

    final text = _controller.text;
    final cursor = _controller.selection.baseOffset;
    final prefixInfo = ParsedLibraryQuery.detectAutocompletePrefix(
      text,
      cursor >= 0 ? cursor : text.length,
    );

    if (prefixInfo == null) {
      _hideOverlay();
      return;
    }

    _currentPrefix = prefixInfo;
    final libraryState = ref.read(libraryProvider);
    final prefixLower = prefixInfo.prefix.toLowerCase();
    final List<AutocompleteSuggestion> matches = [];

    switch (prefixInfo.tag) {
      case 'author':
        for (final author in libraryState.authors) {
          if (prefixLower.isEmpty || author.name.toLowerCase().contains(prefixLower)) {
            matches.add(AutocompleteSuggestion(
              name: author.name,
              tag: 'author',
              icon: Icons.person_outline_rounded,
              count: author.bookCount > 0 ? author.bookCount : null,
            ));
          }
        }
        break;

      case 'genre':
        for (final genre in libraryState.genres) {
          if (prefixLower.isEmpty || genre.name.toLowerCase().contains(prefixLower)) {
            matches.add(AutocompleteSuggestion(
              name: genre.name,
              tag: 'genre',
              icon: Icons.category_outlined,
              count: genre.bookCount > 0 ? genre.bookCount : null,
            ));
          }
        }
        break;

      case 'topic':
        for (final topic in libraryState.topics) {
          if (prefixLower.isEmpty || topic.name.toLowerCase().contains(prefixLower)) {
            matches.add(AutocompleteSuggestion(
              name: topic.name,
              tag: 'topic',
              icon: Icons.label_outline_rounded,
              count: topic.bookCount > 0 ? topic.bookCount : null,
            ));
          }
        }
        break;
    }

    if (matches.isEmpty) {
      _hideOverlay();
      return;
    }

    _suggestions = matches.take(8).toList();
    if (_highlightedIndex >= _suggestions.length) {
      _highlightedIndex = 0;
    }

    _showOverlay();
  }

  void _showOverlay() {
    if (_overlayEntry == null) {
      _overlayEntry = _createOverlayEntry();
      Overlay.of(context).insert(_overlayEntry!);
    } else {
      _overlayEntry?.markNeedsBuild();
    }
  }

  void _hideOverlay() {
    _overlayEntry?.remove();
    _overlayEntry = null;
    _currentPrefix = null;
  }

  void _selectSuggestion(AutocompleteSuggestion item) {
    final prefixInfo = _currentPrefix;
    if (prefixInfo == null) {
      _hideOverlay();
      return;
    }

    final currentText = _controller.text;
    final tokenStart = prefixInfo.tokenStart;
    final tokenEnd = prefixInfo.tokenEnd;

    final before = (tokenStart >= 0 && tokenStart <= currentText.length)
        ? currentText.substring(0, tokenStart)
        : '';
    final after = (tokenEnd >= 0 && tokenEnd <= currentText.length)
        ? currentText.substring(tokenEnd)
        : '';

    final tagFormatted = '${prefixInfo.tag[0].toUpperCase()}${prefixInfo.tag.substring(1)}';
    final replacement = '$tagFormatted:"${item.name}" ';
    final newText = '$before$replacement$after';
    final newCursorOffset = (before + replacement).length;

    _controller.value = TextEditingValue(
      text: newText,
      selection: TextSelection.collapsed(offset: newCursorOffset),
    );

    ref.read(libraryProvider.notifier).setSearchQuery(newText);
    _hideOverlay();
    _focusNode.requestFocus();
  }

  OverlayEntry _createOverlayEntry() {
    final renderBox = context.findRenderObject() as RenderBox?;
    final size = renderBox?.size ?? const Size(400, 48);

    return OverlayEntry(
      builder: (context) {
        return Positioned(
          width: size.width,
          child: CompositedTransformFollower(
            link: _layerLink,
            showWhenUnlinked: false,
            offset: Offset(0, size.height + AppTokens.space4),
            child: Material(
              elevation: 4,
              shadowColor: Colors.black12,
              borderRadius: BorderRadius.circular(AppTokens.radiusMd),
              color: AppTokens.boneSurface,
              child: Container(
                decoration: BoxDecoration(
                  color: AppTokens.boneSurface,
                  borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                  border: Border.all(color: AppTokens.crispBorder),
                ),
                constraints: const BoxConstraints(maxHeight: 280),
                child: ClipRRect(
                  borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                  child: ListView.builder(
                    padding: const EdgeInsets.symmetric(vertical: AppTokens.space4),
                    shrinkWrap: true,
                    itemCount: _suggestions.length,
                    itemBuilder: (context, index) {
                      final item = _suggestions[index];
                      final isHighlighted = index == _highlightedIndex;
                      return InkWell(
                        onTap: () => _selectSuggestion(item),
                        child: Container(
                          color: isHighlighted ? AppTokens.boneContainer : Colors.transparent,
                          padding: const EdgeInsets.symmetric(
                            horizontal: AppTokens.space16,
                            vertical: AppTokens.space8,
                          ),
                          child: Row(
                            children: [
                              Icon(
                                item.icon,
                                size: 16,
                                color: isHighlighted ? AppTokens.charcoalInk : AppTokens.mutedCopy,
                              ),
                              const SizedBox(width: AppTokens.space12),
                              Expanded(
                                child: Text(
                                  item.name,
                                  style: AppTypography.bodySans(
                                    fontSize: 14,
                                    fontWeight: isHighlighted ? FontWeight.w600 : FontWeight.w400,
                                    color: AppTokens.charcoalInk,
                                  ),
                                  overflow: TextOverflow.ellipsis,
                                ),
                              ),
                              if (item.count != null)
                                Text(
                                  '${item.count} ${item.count == 1 ? 'book' : 'books'}',
                                  style: AppTypography.captionSans(
                                    fontSize: 12,
                                    color: AppTokens.mutedCopy,
                                  ),
                                ),
                            ],
                          ),
                        ),
                      );
                    },
                  ),
                ),
              ),
            ),
          ),
        );
      },
    );
  }

  void _clearSearch() {
    _controller.clear();
    ref.read(libraryProvider.notifier).setSearchQuery('');
    _hideOverlay();
    _focusNode.requestFocus();
  }

  @override
  Widget build(BuildContext context) {
    ref.listen<String>(libraryProvider.select((s) => s.searchQuery), (prev, next) {
      if (next != _controller.text) {
        _controller.value = TextEditingValue(
          text: next,
          selection: TextSelection.collapsed(offset: next.length),
        );
      }
    });

    final hasText = _controller.text.isNotEmpty;

    return CompositedTransformTarget(
      link: _layerLink,
      child: TextField(
        controller: _controller,
        focusNode: _focusNode,
        textInputAction: TextInputAction.search,
        style: AppTypography.bodySans(
          fontSize: 14,
          color: AppTokens.charcoalInk,
        ),
        decoration: InputDecoration(
          hintText: widget.hintText ?? 'Filter by title, author:"name", genre:"...", topic:"..."',
          hintStyle: AppTypography.bodySans(
            fontSize: 14,
            color: AppTokens.mutedCopy.withValues(alpha: 0.7),
          ),
          filled: true,
          fillColor: AppTokens.boneSurface,
          prefixIcon: const Icon(
            Icons.search_rounded,
            color: AppTokens.mutedCopy,
            size: 20,
          ),
          suffixIcon: hasText
              ? IconButton(
                  icon: const Icon(Icons.close_rounded, size: 18),
                  tooltip: 'Clear filter',
                  color: AppTokens.mutedCopy,
                  onPressed: _clearSearch,
                )
              : null,
          contentPadding: const EdgeInsets.symmetric(
            horizontal: AppTokens.space16,
            vertical: AppTokens.space12,
          ),
          border: OutlineInputBorder(
            borderRadius: BorderRadius.circular(AppTokens.radiusMd),
            borderSide: const BorderSide(color: AppTokens.crispBorder),
          ),
          enabledBorder: OutlineInputBorder(
            borderRadius: BorderRadius.circular(AppTokens.radiusMd),
            borderSide: const BorderSide(color: AppTokens.crispBorder),
          ),
          focusedBorder: OutlineInputBorder(
            borderRadius: BorderRadius.circular(AppTokens.radiusMd),
            borderSide: const BorderSide(color: AppTokens.charcoalInk, width: 1.5),
          ),
        ),
      ),
    );
  }
}
