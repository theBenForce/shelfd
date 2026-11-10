import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../data/models/author.dart';
import '../../data/models/book.dart';
import '../../data/models/genre.dart';
import '../../data/models/series.dart';
import '../../data/models/topic.dart';
import '../features/audiobook/widgets/mini_player_bar.dart';
import '../state/providers.dart';
import 'responsive.dart';
import 'theme.dart';
import 'tokens.dart';
import 'typography.dart';

/// Resolves a book ID to its human-readable title if found in the loaded catalog.
String? resolveBookTitle(String? bookOrId, List<Book> books) {
  if (bookOrId == null || bookOrId.isEmpty) return bookOrId;
  final match = books.where((b) => b.id == bookOrId).firstOrNull;
  return match != null && match.title.isNotEmpty ? match.title : bookOrId;
}

class BentoCard extends StatelessWidget {
  final Widget child;
  final VoidCallback? onTap;
  final EdgeInsetsGeometry padding;
  final Color? backgroundColor;
  final double? width;
  final double? height;

  const BentoCard({
    super.key,
    required this.child,
    this.onTap,
    this.padding = const EdgeInsets.all(AppTokens.space16),
    this.backgroundColor,
    this.width,
    this.height,
  });

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    return DecoratedBox(
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        boxShadow: [
          BoxShadow(
            color: colors.cardShadowColor,
            blurRadius: 16,
            offset: const Offset(0, 4),
          ),
        ],
      ),
      child: Card(
        elevation: 0,
        margin: EdgeInsets.zero,
        color: backgroundColor ?? colors.surface,
        surfaceTintColor: Colors.transparent,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppTokens.radiusMd),
          side: BorderSide(color: colors.subtleBorder),
        ),
        clipBehavior: Clip.antiAlias,
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(AppTokens.radiusMd),
          child: Container(
            width: width,
            height: height,
            padding: padding,
            child: child,
          ),
        ),
      ),
    );
  }
}

class FilterPillItem {
  final String id;
  final String label;

  const FilterPillItem({required this.id, required this.label});
}

class FilterPillsRow extends StatelessWidget {
  final List<FilterPillItem> items;
  final String selectedId;
  final ValueChanged<String> onSelected;

  const FilterPillsRow({
    super.key,
    required this.items,
    required this.selectedId,
    required this.onSelected,
  });

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: Row(
        children: items.map((item) {
          final isSelected = item.id == selectedId;
          return Padding(
            padding: const EdgeInsets.only(right: AppTokens.space8),
            child: ChoiceChip(
              label: Text(
                item.label,
                style: AppTypography.bodySans(
                  fontSize: 13,
                  fontWeight: isSelected ? FontWeight.w600 : FontWeight.w500,
                  color: isSelected ? colors.onAccent : colors.secondaryInk,
                ),
              ),
              selected: isSelected,
              onSelected: (_) => onSelected(item.id),
              selectedColor: colors.accentPrimary,
              backgroundColor: colors.containerLow,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(AppTokens.radiusPill),
                side: BorderSide(
                  color: isSelected ? colors.accentPrimary : colors.subtleBorder,
                ),
              ),
              showCheckmark: false,
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
            ),
          );
        }).toList(),
      ),
    );
  }
}

class StatusBadge extends StatelessWidget {
  final String label;
  final Color backgroundColor;
  final Color textColor;

  const StatusBadge({
    super.key,
    required this.label,
    this.backgroundColor = AppTokens.matchBadgeBg,
    this.textColor = AppTokens.matchBadgeText,
  });

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    final isDefaultMatchColors =
        backgroundColor == AppTokens.matchBadgeBg &&
        textColor == AppTokens.matchBadgeText;
    final effectiveBg = (colors.isDark && isDefaultMatchColors)
        ? colors.accentSecondaryContainer
        : backgroundColor;
    final effectiveText = (colors.isDark && isDefaultMatchColors)
        ? colors.accentSecondary
        : textColor;

    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppTokens.space8,
        vertical: AppTokens.space4,
      ),
      decoration: BoxDecoration(
        color: effectiveBg,
        borderRadius: BorderRadius.circular(AppTokens.radiusPill),
      ),
      child: Text(
        label,
        style: AppTypography.mono(
          fontSize: 11,
          fontWeight: FontWeight.w600,
          color: effectiveText,
          letterSpacing: 0.2,
        ),
      ),
    );
  }
}

class AuthorAvatar extends StatelessWidget {
  final String name;
  final String? photoUrl;
  final double size;

  const AuthorAvatar({
    super.key,
    required this.name,
    this.photoUrl,
    this.size = 56,
  });

  String get initials {
    final parts = name.trim().split(RegExp(r'\s+'));
    if (parts.isEmpty || parts[0].isEmpty) return '?';
    if (parts.length == 1) return parts[0][0].toUpperCase();
    return (parts[0][0] + parts.last[0]).toUpperCase();
  }

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        color: colors.container,
        shape: BoxShape.circle,
        border: Border.all(color: colors.border, width: 1.2),
        boxShadow: [
          BoxShadow(
            color: colors.cardShadowColor,
            blurRadius: 8,
            offset: const Offset(0, 3),
          ),
        ],
      ),
      clipBehavior: Clip.antiAlias,
      child: photoUrl != null && photoUrl!.isNotEmpty
          ? Image.network(
              photoUrl!,
              fit: BoxFit.cover,
              errorBuilder: (context, error, stackTrace) =>
                  _buildInitials(colors),
            )
          : _buildInitials(colors),
    );
  }

  Widget _buildInitials(ShelfdColors colors) {
    return Center(
      child: Text(
        initials,
        style: AppTypography.titleSerif(
          fontSize: size * 0.38,
          fontWeight: FontWeight.w700,
          color: colors.accentPrimary,
        ),
      ),
    );
  }
}

class PrimaryButton extends StatelessWidget {
  final String label;
  final VoidCallback? onPressed;
  final bool isLoading;
  final IconData? icon;
  final Color? backgroundColor;
  final Color? foregroundColor;

  const PrimaryButton({
    super.key,
    required this.label,
    required this.onPressed,
    this.isLoading = false,
    this.icon,
    this.backgroundColor,
    this.foregroundColor,
  });

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    final effectiveBg = backgroundColor ?? colors.accentPrimary;
    final effectiveFg = foregroundColor ?? colors.onAccent;

    return SizedBox(
      height: AppTokens.minTouchTarget,
      child: ElevatedButton(
        style: ElevatedButton.styleFrom(
          backgroundColor: effectiveBg,
          foregroundColor: effectiveFg,
          elevation: 0,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(AppTokens.radiusSm),
          ),
          padding: const EdgeInsets.symmetric(horizontal: AppTokens.space20),
        ),
        onPressed: isLoading ? null : onPressed,
        child: isLoading
            ? SizedBox(
                width: 20,
                height: 20,
                child: CircularProgressIndicator(
                  strokeWidth: 2,
                  color: effectiveFg,
                ),
              )
            : Row(
                mainAxisAlignment: MainAxisAlignment.center,
                mainAxisSize: MainAxisSize.min,
                children: [
                  if (icon != null) ...[
                    Icon(icon, size: 18),
                    const SizedBox(width: AppTokens.space8),
                  ],
                  Text(
                    label,
                    style: AppTypography.bodySans(
                      fontSize: 15,
                      fontWeight: FontWeight.w600,
                      color: effectiveFg,
                    ),
                  ),
                ],
              ),
      ),
    );
  }
}

class ShelfdTopBar extends StatelessWidget implements PreferredSizeWidget {
  final String title;
  final String? subtitle;
  final List<Widget>? actions;
  final Widget? leading;
  final VoidCallback? onOpenUploads;

  const ShelfdTopBar({
    super.key,
    this.title = 'Shelfd',
    this.subtitle,
    this.actions,
    this.leading,
    this.onOpenUploads,
  });

  @override
  Size get preferredSize => const Size.fromHeight(kToolbarHeight + 8);

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    return AppBar(
      backgroundColor: colors.background,
      foregroundColor: colors.ink,
      surfaceTintColor: Colors.transparent,
      elevation: 0,
      scrolledUnderElevation: 0,
      leading: leading,
      title: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            title,
            style: AppTypography.titleSerif(fontSize: 20, color: colors.ink),
          ),
          if (subtitle != null) ...[
            const SizedBox(height: 2),
            Row(
              children: [
                Container(
                  width: 6,
                  height: 6,
                  decoration: BoxDecoration(
                    color: colors.accentSecondary,
                    shape: BoxShape.circle,
                  ),
                ),
                const SizedBox(width: 4),
                Expanded(
                  child: Text(
                    subtitle!,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: AppTypography.bodySans(
                      fontSize: 11,
                      color: colors.mutedInk,
                    ),
                  ),
                ),
              ],
            ),
          ],
        ],
      ),
      actions: actions ??
          (onOpenUploads != null
              ? [ShelfdUploadsBadgeButton(onPressed: onOpenUploads)]
              : null),
      bottom: PreferredSize(
        preferredSize: const Size.fromHeight(1),
        child: Container(color: colors.subtleBorder, height: 1),
      ),
    );
  }
}

class ShelfdUploadsBadgeButton extends ConsumerWidget {
  final VoidCallback? onPressed;

  const ShelfdUploadsBadgeButton({super.key, this.onPressed});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = ShelfdColors.of(context);
    final queue = ref.watch(queueProvider);
    final count = queue.status?.stagedUploads ?? 0;
    return IconButton(
      tooltip: count > 0 ? '$count staged uploads' : 'Uploads',
      icon: Badge(
        isLabelVisible: count > 0,
        label: Text(
          '$count',
          style: AppTypography.mono(
            fontWeight: FontWeight.w700,
            fontSize: 10,
            color: colors.onAccent,
          ),
        ),
        backgroundColor: colors.accentPrimary,
        textColor: colors.onAccent,
        child: Icon(Icons.cloud_upload_outlined, size: 22, color: colors.ink),
      ),
      onPressed: onPressed ?? () => context.go('/uploads'),
    );
  }
}

class ShelfdBottomNav extends ConsumerWidget {
  final int currentIndex;
  final ValueChanged<int> onTap;
  final String? currentPath;
  final ValueChanged<String>? onNavigate;

  const ShelfdBottomNav({
    super.key,
    required this.currentIndex,
    required this.onTap,
    this.currentPath,
    this.onNavigate,
  });

  int _calculateIndex() {
    if (currentPath != null) {
      final p = currentPath!;
      if (p.startsWith('/series')) return 1;
      if (p.startsWith('/authors') || p.startsWith('/author')) return 2;
      if (p.startsWith('/search')) return 3;
      if (p.startsWith('/uploads')) return 4;
      if (p.startsWith('/utilities')) return 5;
      if (p.startsWith('/settings')) return 6;
      return 0; // /books, /library, default
    }
    return currentIndex.clamp(0, 6);
  }

  void _handleTap(int index) {
    if (onNavigate != null) {
      switch (index) {
        case 0:
          onNavigate!('/books');
          return;
        case 1:
          onNavigate!('/series');
          return;
        case 2:
          onNavigate!('/authors');
          return;
        case 3:
          onNavigate!('/search');
          return;
        case 4:
          onNavigate!('/uploads');
          return;
        case 5:
          onNavigate!('/utilities');
          return;
        case 6:
          onNavigate!('/settings');
          return;
      }
    }
    onTap(index);
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final idx = _calculateIndex();
    final queueStatus = ref.watch(queueProvider).status;
    final stagedCount = queueStatus?.stagedUploads ?? 0;
    final colors = ShelfdColors.of(context);

    return Container(
      decoration: BoxDecoration(
        border: Border(top: BorderSide(color: colors.subtleBorder, width: 1)),
      ),
      child: NavigationBar(
        selectedIndex: idx,
        onDestinationSelected: _handleTap,
        backgroundColor: colors.containerLow,
        surfaceTintColor: Colors.transparent,
        elevation: 0,
        height: 64,
        indicatorColor: colors.containerHigh,
        destinations: [
          NavigationDestination(
            icon: Icon(Icons.menu_book_outlined, color: colors.secondaryInk),
            selectedIcon: Icon(
              Icons.menu_book_rounded,
              color: colors.accentPrimary,
            ),
            label: 'Books',
          ),
          NavigationDestination(
            icon: Icon(
              Icons.collections_bookmark_outlined,
              color: colors.secondaryInk,
            ),
            selectedIcon: Icon(
              Icons.collections_bookmark_rounded,
              color: colors.accentPrimary,
            ),
            label: 'Series',
          ),
          NavigationDestination(
            icon: Icon(
              Icons.people_outline_rounded,
              color: colors.secondaryInk,
            ),
            selectedIcon: Icon(
              Icons.people_rounded,
              color: colors.accentPrimary,
            ),
            label: 'Authors',
          ),
          NavigationDestination(
            icon: Icon(Icons.saved_search_outlined, color: colors.secondaryInk),
            selectedIcon: Icon(
              Icons.saved_search_rounded,
              color: colors.accentPrimary,
            ),
            label: 'Search',
          ),
          NavigationDestination(
            icon: Badge(
              isLabelVisible: stagedCount > 0,
              label: Text(
                '$stagedCount',
                style: AppTypography.mono(
                  fontWeight: FontWeight.w700,
                  fontSize: 10,
                  color: colors.onAccent,
                ),
              ),
              backgroundColor: colors.accentPrimary,
              textColor: colors.onAccent,
              child: Icon(
                Icons.cloud_upload_outlined,
                color: colors.secondaryInk,
              ),
            ),
            selectedIcon: Badge(
              isLabelVisible: stagedCount > 0,
              label: Text(
                '$stagedCount',
                style: AppTypography.mono(
                  fontWeight: FontWeight.w700,
                  fontSize: 10,
                  color: colors.onAccent,
                ),
              ),
              backgroundColor: colors.accentPrimary,
              textColor: colors.onAccent,
              child: Icon(
                Icons.cloud_upload_rounded,
                color: colors.accentPrimary,
              ),
            ),
            label: 'Uploads',
          ),
          NavigationDestination(
            icon: Icon(Icons.handyman_outlined, color: colors.secondaryInk),
            selectedIcon: Icon(
              Icons.handyman_rounded,
              color: colors.accentPrimary,
            ),
            label: 'Utilities',
          ),
          NavigationDestination(
            icon: Icon(Icons.settings_outlined, color: colors.secondaryInk),
            selectedIcon: Icon(
              Icons.settings_rounded,
              color: colors.accentPrimary,
            ),
            label: 'Settings',
          ),
        ],
      ),
    );
  }
}

class ShelfdSideNav extends ConsumerStatefulWidget {
  final int currentIndex;
  final ValueChanged<int> onTap;
  final String? currentPath;
  final ValueChanged<String>? onNavigate;
  final VoidCallback? onRescan;
  final VoidCallback? onUpload;
  final bool isRescanning;

  const ShelfdSideNav({
    super.key,
    required this.currentIndex,
    required this.onTap,
    this.currentPath,
    this.onNavigate,
    this.onRescan,
    this.onUpload,
    this.isRescanning = false,
  });

  @override
  ConsumerState<ShelfdSideNav> createState() => _ShelfdSideNavState();
}

class _ShelfdSideNavState extends ConsumerState<ShelfdSideNav> {
  bool _isLibraryExpanded = true;

  void _navigate(String path, int fallbackIndex) {
    if (widget.onNavigate != null) {
      widget.onNavigate!(path);
    } else {
      widget.onTap(fallbackIndex);
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    final p = widget.currentPath ?? '';
    final isBooks =
        p.startsWith('/books') || p == '/library' || p.startsWith('/book/');
    final isSeries = p.startsWith('/series');
    final isAuthors = p.startsWith('/authors') || p.startsWith('/author');
    final isGenres = p.startsWith('/genres') || p.startsWith('/genre');
    final isTopics = p.startsWith('/topics') || p.startsWith('/topic');
    final isSearch =
        p.startsWith('/search') ||
        (widget.currentPath == null && widget.currentIndex == 1);
    final isUploads = p.startsWith('/uploads');
    final isUtilities = p.startsWith('/utilities');
    final isSettings =
        p.startsWith('/settings') ||
        (widget.currentPath == null && widget.currentIndex == 2);
    final isLibraryActive =
        isBooks ||
        isSeries ||
        isAuthors ||
        isGenres ||
        isTopics ||
        (widget.currentPath == null && widget.currentIndex == 0);
    final queueStatus = ref.watch(queueProvider).status;
    final stagedCount = queueStatus?.stagedUploads ?? 0;

    return Container(
      width: AppTokens.sidebarWidth,
      height: double.infinity,
      decoration: BoxDecoration(
        color: colors.containerLow,
        border: Border(
          right: BorderSide(color: colors.subtleBorder, width: 1),
        ),
      ),
      padding: const EdgeInsets.symmetric(
        horizontal: AppTokens.space20,
        vertical: AppTokens.space24,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Brand Logo & Title
          Row(
            children: [
              Container(
                width: 36,
                height: 36,
                decoration: BoxDecoration(
                  color: colors.accentPrimary,
                  borderRadius: BorderRadius.circular(AppTokens.radiusSm),
                ),
                child: Icon(
                  Icons.auto_stories_rounded,
                  color: colors.onAccent,
                  size: 20,
                ),
              ),
              const SizedBox(width: AppTokens.space12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(
                      'Shelfd',
                      style: AppTypography.titleSerif(
                        fontSize: 22,
                        fontWeight: FontWeight.w700,
                        color: colors.ink,
                      ),
                    ),
                    Text(
                      'Your Digital Vellum',
                      style: AppTypography.bodySans(
                        fontSize: 11,
                        color: colors.mutedInk,
                      ),
                      overflow: TextOverflow.ellipsis,
                    ),
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: AppTokens.space24),

          // Scrollable Navigation Links
          Expanded(
            child: SingleChildScrollView(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // Collapsible Library Header Item
                  _SideNavItem(
                    icon: Icons.local_library_outlined,
                    selectedIcon: Icons.local_library_rounded,
                    label: 'Library',
                    isSelected: isLibraryActive && !_isLibraryExpanded,
                    trailing: IconButton(
                      visualDensity: VisualDensity.compact,
                      icon: Icon(
                        _isLibraryExpanded
                            ? Icons.keyboard_arrow_down_rounded
                            : Icons.keyboard_arrow_right_rounded,
                        size: 20,
                        color: colors.mutedInk,
                      ),
                      onPressed: () {
                        setState(() {
                          _isLibraryExpanded = !_isLibraryExpanded;
                        });
                      },
                    ),
                    onTap: () {
                      setState(() {
                        _isLibraryExpanded = !_isLibraryExpanded;
                      });
                      if (_isLibraryExpanded) {
                        _navigate('/books', 0);
                      }
                    },
                  ),

                  // Indented Sub-items under Library
                  if (_isLibraryExpanded) ...[
                    _SideNavSubItem(
                      icon: Icons.menu_book_outlined,
                      selectedIcon: Icons.menu_book_rounded,
                      label: 'Books',
                      isSelected:
                          isBooks ||
                          (isLibraryActive &&
                              !isSeries &&
                              !isAuthors &&
                              !isGenres &&
                              !isTopics),
                      onTap: () => _navigate('/books', 0),
                    ),
                    _SideNavSubItem(
                      icon: Icons.collections_bookmark_outlined,
                      selectedIcon: Icons.collections_bookmark_rounded,
                      label: 'Series',
                      isSelected: isSeries,
                      onTap: () => _navigate('/series', 0),
                    ),
                    _SideNavSubItem(
                      icon: Icons.people_outline_rounded,
                      selectedIcon: Icons.people_rounded,
                      label: 'Authors',
                      isSelected: isAuthors,
                      onTap: () => _navigate('/authors', 0),
                    ),
                    _SideNavSubItem(
                      icon: Icons.category_outlined,
                      selectedIcon: Icons.category_rounded,
                      label: 'Genres',
                      isSelected: isGenres,
                      onTap: () => _navigate('/genres', 0),
                    ),
                    _SideNavSubItem(
                      icon: Icons.label_outline_rounded,
                      selectedIcon: Icons.label_rounded,
                      label: 'Topics',
                      isSelected: isTopics,
                      onTap: () => _navigate('/topics', 0),
                    ),
                  ],
                  const SizedBox(height: AppTokens.space8),

                  // Top-level Navigation Links
                  _SideNavItem(
                    icon: Icons.saved_search_outlined,
                    selectedIcon: Icons.saved_search_rounded,
                    label: 'Semantic Search',
                    isSelected: isSearch,
                    onTap: () => _navigate('/search', 3),
                  ),
                  const SizedBox(height: AppTokens.space8),
                  _SideNavItem(
                    icon: Icons.cloud_upload_outlined,
                    selectedIcon: Icons.cloud_upload_rounded,
                    label: 'Uploads',
                    isSelected: isUploads,
                    trailing: stagedCount > 0
                        ? Container(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 7,
                              vertical: 2,
                            ),
                            decoration: BoxDecoration(
                              color: colors.accentPrimary.withValues(
                                alpha: 0.14,
                              ),
                              border: Border.all(
                                color: colors.accentPrimary.withValues(
                                  alpha: 0.35,
                                ),
                              ),
                              borderRadius: BorderRadius.circular(10),
                            ),
                            child: Text(
                              '$stagedCount',
                              style: AppTypography.mono(
                                fontSize: 11,
                                fontWeight: FontWeight.w700,
                                color: colors.accentPrimary,
                              ),
                            ),
                          )
                        : null,
                    onTap: () => _navigate('/uploads', 4),
                  ),
                  const SizedBox(height: AppTokens.space8),
                  _SideNavItem(
                    icon: Icons.handyman_outlined,
                    selectedIcon: Icons.handyman_rounded,
                    label: 'Utilities',
                    isSelected: isUtilities,
                    onTap: () => _navigate('/utilities', 5),
                  ),
                  const SizedBox(height: AppTokens.space8),
                  _SideNavItem(
                    icon: Icons.settings_outlined,
                    selectedIcon: Icons.settings_rounded,
                    label: 'Settings',
                    isSelected: isSettings,
                    onTap: () => _navigate('/settings', 6),
                  ),
                ],
              ),
            ),
          ),

          const SizedBox(height: AppTokens.space12),

          // Live AI indexing queue progress card
          const _QueueStatusCard(),
          const SizedBox(height: AppTokens.space12),

          // Rescan Library Button (if callback provided)
          if (widget.onRescan != null) ...[
            SizedBox(
              width: double.infinity,
              height: 40,
              child: OutlinedButton.icon(
                style: OutlinedButton.styleFrom(
                  backgroundColor: colors.accentPrimary,
                  foregroundColor: colors.onAccent,
                  side: BorderSide.none,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                  ),
                  padding: const EdgeInsets.symmetric(
                    horizontal: AppTokens.space12,
                  ),
                ),
                onPressed: widget.isRescanning ? null : widget.onRescan,
                icon: widget.isRescanning
                    ? SizedBox(
                        width: 16,
                        height: 16,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          color: colors.onAccent,
                        ),
                      )
                    : const Icon(Icons.sync_rounded, size: 18),
                label: Text(
                  widget.isRescanning ? 'Scanning...' : 'Rescan Library',
                  style: AppTypography.bodySans(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: colors.onAccent,
                  ),
                ),
              ),
            ),
            const SizedBox(height: AppTokens.space16),
          ],

          // Homelab NAS connection status badge
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
            decoration: BoxDecoration(
              color: colors.container,
              borderRadius: BorderRadius.circular(AppTokens.radiusSm),
              border: Border.all(color: colors.subtleBorder),
            ),
            child: Row(
              children: [
                Container(
                  width: 8,
                  height: 8,
                  decoration: BoxDecoration(
                    color: colors.accentSecondary,
                    shape: BoxShape.circle,
                  ),
                ),
                const SizedBox(width: AppTokens.space8),
                Expanded(
                  child: Text(
                    'Homelab NAS - Connected',
                    style: AppTypography.bodySans(
                      fontSize: 11,
                      fontWeight: FontWeight.w500,
                      color: colors.secondaryInk,
                    ),
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _SideNavItem extends StatelessWidget {
  final IconData icon;
  final IconData selectedIcon;
  final String label;
  final bool isSelected;
  final VoidCallback onTap;
  final Widget? trailing;

  const _SideNavItem({
    required this.icon,
    required this.selectedIcon,
    required this.label,
    required this.isSelected,
    required this.onTap,
    this.trailing,
  });

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    final selectedBg = colors.containerHigh;
    final unselectedColor = colors.secondaryInk;
    final activeColor = colors.ink;

    return Material(
      color: isSelected ? selectedBg : Colors.transparent,
      borderRadius: BorderRadius.circular(AppTokens.radiusMd),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        child: Container(
          height: AppTokens.minTouchTarget,
          padding: const EdgeInsets.symmetric(horizontal: AppTokens.space12),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(AppTokens.radiusMd),
            border: isSelected ? Border.all(color: colors.subtleBorder) : null,
          ),
          child: Row(
            children: [
              Icon(
                isSelected ? selectedIcon : icon,
                color: isSelected ? colors.accentPrimary : unselectedColor,
                size: 22,
              ),
              const SizedBox(width: AppTokens.space12),
              Expanded(
                child: Text(
                  label,
                  style: AppTypography.bodySans(
                    fontSize: 14,
                    fontWeight: isSelected ? FontWeight.w600 : FontWeight.w400,
                    color: isSelected ? activeColor : unselectedColor,
                  ),
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              ?trailing,
              if (isSelected && trailing == null)
                Container(
                  width: 4,
                  height: 16,
                  decoration: BoxDecoration(
                    color: colors.accentPrimary,
                    borderRadius: BorderRadius.circular(2),
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }
}

class _SideNavSubItem extends StatelessWidget {
  final IconData icon;
  final IconData selectedIcon;
  final String label;
  final bool isSelected;
  final VoidCallback onTap;

  const _SideNavSubItem({
    required this.icon,
    required this.selectedIcon,
    required this.label,
    required this.isSelected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    return Padding(
      padding: const EdgeInsets.only(left: AppTokens.space16, top: 2, bottom: 2),
      child: Material(
        color: isSelected ? colors.container : Colors.transparent,
        borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(AppTokens.radiusMd),
          child: Container(
            height: 36,
            padding: const EdgeInsets.symmetric(horizontal: AppTokens.space12),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(AppTokens.radiusMd),
              border: isSelected
                  ? Border.all(color: colors.subtleBorder)
                  : null,
            ),
            child: Row(
              children: [
                Icon(
                  isSelected ? selectedIcon : icon,
                  color: isSelected ? colors.accentPrimary : colors.mutedInk,
                  size: 18,
                ),
                const SizedBox(width: AppTokens.space8),
                Expanded(
                  child: Text(
                    label,
                    style: AppTypography.bodySans(
                      fontSize: 13,
                      fontWeight: isSelected
                          ? FontWeight.w600
                          : FontWeight.w400,
                      color: isSelected ? colors.ink : colors.secondaryInk,
                    ),
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
                if (isSelected)
                  Container(
                    width: 4,
                    height: 14,
                    decoration: BoxDecoration(
                      color: colors.accentPrimary,
                      borderRadius: BorderRadius.circular(2),
                    ),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _QueueStatusCard extends ConsumerWidget {
  const _QueueStatusCard();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = ShelfdColors.of(context);
    final queueState = ref.watch(queueProvider);
    final status = queueState.status;

    if (status == null) {
      if (queueState.isLoading) {
        return Container(
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
          decoration: BoxDecoration(
            color: colors.container.withValues(alpha: 0.6),
            borderRadius: BorderRadius.circular(AppTokens.radiusSm),
            border: Border.all(color: colors.subtleBorder),
          ),
          child: Row(
            children: [
              SizedBox(
                width: 12,
                height: 12,
                child: CircularProgressIndicator(
                  strokeWidth: 1.5,
                  color: colors.mutedInk,
                ),
              ),
              const SizedBox(width: AppTokens.space8),
              Expanded(
                child: Text(
                  'Checking queue...',
                  style: AppTypography.bodySans(
                    fontSize: 11,
                    color: colors.mutedInk,
                  ),
                  overflow: TextOverflow.ellipsis,
                ),
              ),
            ],
          ),
        );
      }
      return const SizedBox.shrink();
    }

    final isActive =
        status.isActive ||
        status.pendingChapters > 0 ||
        status.pendingUploads > 0;

    if (!isActive) {
      return Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
        decoration: BoxDecoration(
          color: colors.container.withValues(alpha: 0.6),
          borderRadius: BorderRadius.circular(AppTokens.radiusSm),
          border: Border.all(color: colors.subtleBorder),
        ),
        child: Row(
          children: [
            Icon(
              Icons.check_circle_outline_rounded,
              size: 14,
              color: colors.accentSecondary,
            ),
            const SizedBox(width: AppTokens.space8),
            Expanded(
              child: Text(
                'AI Catalog Synced (${status.totalChapters} passages)',
                style: AppTypography.bodySans(
                  fontSize: 11,
                  fontWeight: FontWeight.w500,
                  color: colors.mutedInk,
                ),
                overflow: TextOverflow.ellipsis,
              ),
            ),
          ],
        ),
      );
    }

    final libraryState = ref.watch(libraryProvider);
    final resolvedBook = resolveBookTitle(
      status.currentBook,
      libraryState.books,
    );
    final effectivePercent = status.progressPercent > 0
        ? status.progressPercent
        : (status.totalChapters > 0 &&
                  status.pendingChapters < status.totalChapters
              ? ((status.totalChapters - status.pendingChapters) /
                        status.totalChapters) *
                    100.0
              : 0.0);
    final progress = (effectivePercent / 100.0).clamp(0.0, 1.0);
    final title = (resolvedBook != null && resolvedBook.isNotEmpty)
        ? resolvedBook
        : (status.pendingUploads > 0
              ? 'Processing uploads...'
              : 'Indexing library passages...');

    final subtext =
        (status.currentChapter != null && status.currentChapter!.isNotEmpty)
        ? status.currentChapter!
        : '${status.indexedChapters} of ${status.totalChapters} indexed (${effectivePercent.toStringAsFixed(0)}%)';

    return InkWell(
      onTap: () => ref.read(queueProvider.notifier).refresh(),
      borderRadius: BorderRadius.circular(AppTokens.radiusMd),
      child: Container(
        padding: const EdgeInsets.all(AppTokens.space12),
        decoration: BoxDecoration(
          color: colors.surface,
          borderRadius: BorderRadius.circular(AppTokens.radiusMd),
          border: Border.all(color: colors.subtleBorder),
          boxShadow: [
            BoxShadow(
              color: colors.cardShadowColor,
              blurRadius: 8,
              offset: const Offset(0, 2),
            ),
          ],
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Row(
              children: [
                Container(
                  padding: const EdgeInsets.all(4),
                  decoration: BoxDecoration(
                    color: colors.accentPrimary.withValues(alpha: 0.14),
                    borderRadius: BorderRadius.circular(4),
                  ),
                  child: Icon(
                    Icons.auto_awesome_rounded,
                    size: 14,
                    color: colors.accentPrimary,
                  ),
                ),
                const SizedBox(width: AppTokens.space8),
                Expanded(
                  child: Text(
                    'AI Indexing',
                    style: AppTypography.bodySans(
                      fontSize: 12,
                      fontWeight: FontWeight.w600,
                      color: colors.ink,
                    ),
                  ),
                ),
                Text(
                  '${effectivePercent.toStringAsFixed(0)}%',
                  style: AppTypography.mono(
                    fontSize: 11,
                    fontWeight: FontWeight.w700,
                    color: colors.accentPrimary,
                  ),
                ),
              ],
            ),
            const SizedBox(height: AppTokens.space8),
            ClipRRect(
              borderRadius: BorderRadius.circular(2),
              child: LinearProgressIndicator(
                value: progress,
                minHeight: 4,
                backgroundColor: colors.container,
                valueColor: AlwaysStoppedAnimation<Color>(colors.accentPrimary),
              ),
            ),
            const SizedBox(height: AppTokens.space8),
            Text(
              title,
              style: AppTypography.bodySans(
                fontSize: 11,
                fontWeight: FontWeight.w600,
                color: colors.ink,
              ),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
            const SizedBox(height: 2),
            Text(
              subtext,
              style: AppTypography.bodySans(
                fontSize: 10,
                color: colors.mutedInk,
              ),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
          ],
        ),
      ),
    );
  }
}

class ShelfdAdaptiveScaffold extends StatelessWidget {
  final int currentIndex;
  final ValueChanged<int> onNavTap;
  final String? currentPath;
  final ValueChanged<String>? onNavigate;
  final PreferredSizeWidget? appBar;
  final Widget body;
  final VoidCallback? onRescan;
  final VoidCallback? onUpload;
  final bool isRescanning;

  const ShelfdAdaptiveScaffold({
    super.key,
    required this.currentIndex,
    required this.onNavTap,
    this.currentPath,
    this.onNavigate,
    this.appBar,
    required this.body,
    this.onRescan,
    this.onUpload,
    this.isRescanning = false,
  });

  @override
  Widget build(BuildContext context) {
    final isDesktop = Responsive.isDesktop(context);
    final colors = ShelfdColors.of(context);

    if (isDesktop) {
      return Scaffold(
        backgroundColor: colors.background,
        body: Row(
          children: [
            ShelfdSideNav(
              currentIndex: currentIndex,
              onTap: onNavTap,
              currentPath: currentPath,
              onNavigate: onNavigate,
              onRescan: onRescan,
              onUpload: onUpload,
              isRescanning: isRescanning,
            ),
            Expanded(
              child: Column(
                children: [
                  Expanded(child: body),
                  const MiniPlayerBar(),
                ],
              ),
            ),
          ],
        ),
      );
    }

    return Scaffold(
      backgroundColor: colors.background,
      appBar: appBar,
      body: Column(
        children: [
          Expanded(child: body),
          const MiniPlayerBar(),
        ],
      ),
      bottomNavigationBar: ShelfdBottomNav(
        currentIndex: currentIndex,
        onTap: onNavTap,
        currentPath: currentPath,
        onNavigate: onNavigate,
      ),
    );
  }
}

/// Tactile 3D book cover with left-edge book-spine crease, ambient shadow,
/// multi-volume series stack backing plates, and warm editorial cloth fallback.
class TactileBookCover extends StatelessWidget {
  final String? coverUrl;
  final String title;
  final String? subtitle;
  final Map<String, String>? headers;
  final BorderRadiusGeometry borderRadius;
  final double? aspectRatio;
  final double? width;
  final double? height;
  final bool showSpineCrease;
  final bool showShadow;
  final bool isSeries;
  final bool showFallbackTitle;
  final Widget? topRightBadge;
  final Widget? topLeftBadge;
  final Widget? bottomOverlay;
  final IconData fallbackIcon;

  const TactileBookCover({
    super.key,
    this.coverUrl,
    required this.title,
    this.subtitle,
    this.headers,
    this.borderRadius = const BorderRadius.all(
      Radius.circular(AppTokens.radiusSm),
    ),
    this.aspectRatio,
    this.width,
    this.height,
    this.showSpineCrease = true,
    this.showShadow = true,
    this.isSeries = false,
    this.showFallbackTitle = true,
    this.topRightBadge,
    this.topLeftBadge,
    this.bottomOverlay,
    this.fallbackIcon = Icons.auto_stories_rounded,
  });

  @override
  Widget build(BuildContext context) {
    final colors = ShelfdColors.of(context);
    final hasCover = coverUrl != null && coverUrl!.trim().isNotEmpty;

    Widget coverStack = Stack(
      fit: StackFit.expand,
      children: [
        if (isSeries) ...[
          Positioned(
            top: 4,
            bottom: 4,
            left: 8,
            right: 0,
            child: DecoratedBox(
              decoration: BoxDecoration(
                color: colors.containerHigh,
                borderRadius: borderRadius,
                border: Border.all(color: colors.border, width: 1),
              ),
            ),
          ),
          Positioned(
            top: 2,
            bottom: 2,
            left: 4,
            right: 3,
            child: DecoratedBox(
              decoration: BoxDecoration(
                color: colors.container,
                borderRadius: borderRadius,
                border: Border.all(color: colors.border, width: 1),
              ),
            ),
          ),
        ],
        Padding(
          padding: isSeries
              ? const EdgeInsets.only(right: 6)
              : EdgeInsets.zero,
          child: DecoratedBox(
            decoration: BoxDecoration(
              color: colors.container,
              borderRadius: borderRadius,
              border: Border.all(color: colors.subtleBorder, width: 1),
              boxShadow: showShadow
                  ? [
                      BoxShadow(
                        color: colors.cardShadowColor,
                        blurRadius: 14,
                        offset: const Offset(0, 6),
                      ),
                      BoxShadow(
                        color: colors.cardShadowColor.withValues(
                          alpha: colors.isDark ? 0.36 : 0.12,
                        ),
                        blurRadius: 3,
                        offset: const Offset(2, 2),
                      ),
                    ]
                  : null,
            ),
            child: ClipRRect(
              borderRadius: borderRadius,
              child: Stack(
                fit: StackFit.expand,
                children: [
                  hasCover
                      ? Image.network(
                          coverUrl!,
                          headers: headers,
                          fit: BoxFit.cover,
                          width: double.infinity,
                          height: double.infinity,
                          errorBuilder: (context, error, stackTrace) =>
                              _buildEditorialFallback(colors),
                        )
                      : _buildEditorialFallback(colors),
                  if (showSpineCrease)
                    Positioned.fill(
                      child: IgnorePointer(
                        child: DecoratedBox(
                          decoration: BoxDecoration(
                            gradient: LinearGradient(
                              begin: Alignment.centerLeft,
                              end: Alignment.centerRight,
                              stops: const [
                                0.0,
                                0.025,
                                0.055,
                                0.085,
                                0.12,
                                1.0,
                              ],
                              colors: [
                                Colors.black.withValues(alpha: 0.28),
                                Colors.white.withValues(alpha: 0.22),
                                Colors.black.withValues(alpha: 0.18),
                                Colors.white.withValues(alpha: 0.08),
                                Colors.transparent,
                                Colors.transparent,
                              ],
                            ),
                          ),
                        ),
                      ),
                    ),
                  if (topLeftBadge != null)
                    Positioned(
                      top: AppTokens.space8,
                      left: AppTokens.space8,
                      child: topLeftBadge!,
                    ),
                  if (topRightBadge != null)
                    Positioned(
                      top: AppTokens.space8,
                      right: AppTokens.space8,
                      child: topRightBadge!,
                    ),
                  if (bottomOverlay != null)
                    Positioned(
                      left: 0,
                      right: 0,
                      bottom: 0,
                      child: bottomOverlay!,
                    ),
                ],
              ),
            ),
          ),
        ),
      ],
    );

    if (aspectRatio != null) {
      coverStack = AspectRatio(aspectRatio: aspectRatio!, child: coverStack);
    }
    if (width != null || height != null) {
      coverStack = SizedBox(width: width, height: height, child: coverStack);
    }
    return coverStack;
  }

  Widget _buildEditorialFallback(ShelfdColors colors) {
    return LayoutBuilder(
      builder: (context, outerConstraints) {
        final isTiny =
            outerConstraints.maxWidth < 60 || outerConstraints.maxHeight < 75;
        return Container(
          decoration: BoxDecoration(
            gradient: LinearGradient(
              begin: Alignment.topLeft,
              end: Alignment.bottomRight,
              colors: [colors.container, colors.containerHigh],
            ),
          ),
          padding: EdgeInsets.all(isTiny ? 4 : 8),
          child: Container(
            padding: EdgeInsets.symmetric(
              horizontal: isTiny ? 2 : 8,
              vertical: isTiny ? 2 : 8,
            ),
            decoration: BoxDecoration(
              border: Border.all(
                color: colors.accentPrimary.withValues(alpha: 0.28),
                width: 1,
              ),
              borderRadius: BorderRadius.circular(isTiny ? 2 : 4),
            ),
            child: LayoutBuilder(
              builder: (context, constraints) {
                final isCompact = constraints.maxHeight < 95;
                if (isTiny && !showFallbackTitle) {
                  return Center(
                    child: Icon(
                      isSeries
                          ? Icons.collections_bookmark_rounded
                          : fallbackIcon,
                      size: 16,
                      color: colors.accentPrimary.withValues(alpha: 0.75),
                    ),
                  );
                }
                return Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    if (!isCompact || !showFallbackTitle) ...[
                      Icon(
                        isSeries
                            ? Icons.collections_bookmark_rounded
                            : fallbackIcon,
                        size: showFallbackTitle ? 20 : 28,
                        color: colors.accentPrimary.withValues(alpha: 0.75),
                      ),
                      const SizedBox(height: 6),
                      Container(
                        width: 24,
                        height: 1,
                        color: colors.accentPrimary.withValues(alpha: 0.35),
                      ),
                      if (showFallbackTitle) const SizedBox(height: 6),
                    ],
                    if (showFallbackTitle) ...[
                      Flexible(
                        child: Text(
                          title,
                          maxLines: isCompact ? 2 : 3,
                          textAlign: TextAlign.center,
                          overflow: TextOverflow.ellipsis,
                          style: AppTypography.titleSerif(
                            fontSize: isTiny ? 9 : 12,
                            fontWeight: FontWeight.w600,
                            color: colors.ink,
                            height: 1.2,
                          ),
                        ),
                      ),
                      if (!isCompact &&
                          subtitle != null &&
                          subtitle!.trim().isNotEmpty) ...[
                        const SizedBox(height: 4),
                        Text(
                          subtitle!,
                          maxLines: 1,
                          textAlign: TextAlign.center,
                          overflow: TextOverflow.ellipsis,
                          style: AppTypography.captionSans(
                            fontSize: 10,
                            color: colors.mutedInk,
                          ),
                        ),
                      ],
                    ],
                  ],
                );
              },
            ),
          ),
        );
      },
    );
  }
}

/// Unified, polymorphic grid card component for [LibraryGridItem].
class ShelfdGridCard extends StatelessWidget {
  final LibraryGridItem item;
  final VoidCallback onTap;

  const ShelfdGridCard({
    super.key,
    required this.item,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    switch (item) {
      case BookGridItem(:final book):
        return _buildBookCard(context, book);
      case SeriesGridItem(:final series):
        return _buildSeriesCard(context, series);
      case AuthorGridItem(:final author):
        return _buildAuthorCard(context, author);
      case GenreGridItem(:final genre):
        return _buildGenreCard(context, genre);
      case TopicGridItem(:final topic):
        return _buildTopicCard(context, topic);
    }
  }

  Widget _buildBookCard(BuildContext context, Book book) {
    final colors = ShelfdColors.of(context);
    final progressPercent = (book.readingProgress * 100).round();

    return BentoCard(
      onTap: onTap,
      padding: EdgeInsets.zero,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Expanded(
            child: TactileBookCover(
              coverUrl: book.coverUrl,
              title: book.title,
              showFallbackTitle: false,
              borderRadius: const BorderRadius.vertical(
                top: Radius.circular(AppTokens.radiusMd),
              ),
              showSpineCrease: true,
              showShadow: false,
              bottomOverlay: progressPercent > 0
                  ? LinearProgressIndicator(
                      value: book.readingProgress.clamp(0.0, 1.0),
                      minHeight: 3,
                      backgroundColor: colors.containerHigh.withValues(
                        alpha: 0.6,
                      ),
                      valueColor: AlwaysStoppedAnimation<Color>(
                        colors.accentPrimary,
                      ),
                    )
                  : null,
            ),
          ),
          Padding(
            padding: const EdgeInsets.all(AppTokens.space12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  book.title,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.titleSerif(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                    color: colors.ink,
                  ),
                ),
                const SizedBox(height: AppTokens.space4),
                Text(
                  book.authorDisplay,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.bodySans(
                    fontSize: 12,
                    color: colors.secondaryInk,
                  ),
                ),
                if (progressPercent > 0) ...[
                  const SizedBox(height: AppTokens.space8),
                  StatusBadge(
                    label: '$progressPercent% read',
                    backgroundColor: colors.accentSecondaryContainer,
                    textColor: colors.accentSecondary,
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSeriesCard(BuildContext context, Series series) {
    final colors = ShelfdColors.of(context);
    final bookCountLabel = series.bookCount == 1
        ? '1 Book'
        : '${series.bookCount} Books';

    return BentoCard(
      onTap: onTap,
      padding: EdgeInsets.zero,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Expanded(
            child: TactileBookCover(
              coverUrl: series.coverUrl,
              title: series.name,
              isSeries: true,
              showFallbackTitle: false,
              showSpineCrease: true,
              showShadow: false,
              fallbackIcon: Icons.collections_bookmark_rounded,
              borderRadius: const BorderRadius.vertical(
                top: Radius.circular(AppTokens.radiusMd),
              ),
              topRightBadge: Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                decoration: BoxDecoration(
                  color: AppTokens.espressoInk.withValues(alpha: 0.88),
                  borderRadius: BorderRadius.circular(AppTokens.radiusPill),
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const Icon(
                      Icons.collections_bookmark_outlined,
                      size: 12,
                      color: Colors.white,
                    ),
                    const SizedBox(width: 4),
                    Text(
                      bookCountLabel,
                      style: AppTypography.mono(
                        color: Colors.white,
                        fontSize: 11,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
          Padding(
            padding: const EdgeInsets.all(AppTokens.space12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  series.name,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.titleSerif(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                    color: colors.ink,
                  ),
                ),
                const SizedBox(height: AppTokens.space4),
                Text(
                  'Series',
                  style: AppTypography.captionSans(
                    fontSize: 11,
                    color: colors.mutedInk,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildAuthorCard(BuildContext context, Author author) {
    final colors = ShelfdColors.of(context);
    final bookCountLabel = author.bookCount == 1
        ? '1 Book'
        : '${author.bookCount} Books';

    return BentoCard(
      onTap: onTap,
      padding: const EdgeInsets.all(AppTokens.space12),
      child: LayoutBuilder(
        builder: (context, constraints) {
          final isCompact = constraints.maxHeight < 170;
          return Column(
            mainAxisAlignment: MainAxisAlignment.center,
            crossAxisAlignment: CrossAxisAlignment.center,
            mainAxisSize: MainAxisSize.min,
            children: [
              AuthorAvatar(
                name: author.name,
                photoUrl: author.photoUrl,
                size: isCompact ? 52 : 72,
              ),
              SizedBox(height: isCompact ? AppTokens.space8 : AppTokens.space12),
              Flexible(
                child: Text(
                  author.name,
                  maxLines: 2,
                  textAlign: TextAlign.center,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.titleSerif(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                    color: colors.ink,
                  ),
                ),
              ),
              const SizedBox(height: AppTokens.space8),
              StatusBadge(
                label: bookCountLabel,
                backgroundColor: colors.container,
                textColor: colors.secondaryInk,
              ),
            ],
          );
        },
      ),
    );
  }

  Widget _buildGenreCard(BuildContext context, Genre genre) {
    final colors = ShelfdColors.of(context);
    final bookCountLabel = genre.bookCount == 1
        ? '1 Book'
        : '${genre.bookCount} Books';

    return BentoCard(
      onTap: onTap,
      padding: const EdgeInsets.all(AppTokens.space12),
      child: LayoutBuilder(
        builder: (context, constraints) {
          final isCompact = constraints.maxHeight < 150;
          final circleSize = isCompact ? 44.0 : 56.0;
          return Column(
            mainAxisAlignment: MainAxisAlignment.center,
            crossAxisAlignment: CrossAxisAlignment.center,
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: circleSize,
                height: circleSize,
                decoration: BoxDecoration(
                  color: colors.container,
                  shape: BoxShape.circle,
                  border: Border.all(color: colors.subtleBorder),
                ),
                child: Icon(
                  Icons.category_outlined,
                  size: isCompact ? 22 : 26,
                  color: colors.accentPrimary,
                ),
              ),
              SizedBox(height: isCompact ? AppTokens.space8 : AppTokens.space12),
              Flexible(
                child: Text(
                  genre.name,
                  maxLines: 2,
                  textAlign: TextAlign.center,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.titleSerif(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                    color: colors.ink,
                  ),
                ),
              ),
              const SizedBox(height: AppTokens.space8),
              StatusBadge(
                label: bookCountLabel,
                backgroundColor: colors.container,
                textColor: colors.secondaryInk,
              ),
            ],
          );
        },
      ),
    );
  }

  Widget _buildTopicCard(BuildContext context, Topic topic) {
    final colors = ShelfdColors.of(context);
    final bookCountLabel = topic.bookCount == 1
        ? '1 Book'
        : '${topic.bookCount} Books';

    return BentoCard(
      onTap: onTap,
      padding: const EdgeInsets.all(AppTokens.space12),
      child: LayoutBuilder(
        builder: (context, constraints) {
          final isCompact = constraints.maxHeight < 150;
          final circleSize = isCompact ? 44.0 : 56.0;
          return Column(
            mainAxisAlignment: MainAxisAlignment.center,
            crossAxisAlignment: CrossAxisAlignment.center,
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: circleSize,
                height: circleSize,
                decoration: BoxDecoration(
                  color: colors.container,
                  shape: BoxShape.circle,
                  border: Border.all(color: colors.subtleBorder),
                ),
                child: Icon(
                  Icons.label_outline_rounded,
                  size: isCompact ? 22 : 26,
                  color: colors.accentPrimary,
                ),
              ),
              SizedBox(height: isCompact ? AppTokens.space8 : AppTokens.space12),
              Flexible(
                child: Text(
                  topic.name,
                  maxLines: 2,
                  textAlign: TextAlign.center,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.titleSerif(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                    color: colors.ink,
                  ),
                ),
              ),
              const SizedBox(height: AppTokens.space8),
              StatusBadge(
                label: bookCountLabel,
                backgroundColor: colors.container,
                textColor: colors.secondaryInk,
              ),
            ],
          );
        },
      ),
    );
  }
}

void showShelfdSettingsModal(
  BuildContext context, {
  Future<void> Function()? onLogout,
}) {
  final modalColors = ShelfdColors.of(context);
  showModalBottomSheet(
    context: context,
    isScrollControlled: true,
    backgroundColor: modalColors.surface,
    shape: const RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(
        top: Radius.circular(AppTokens.radiusLg),
      ),
    ),
    builder: (ctx) {
      final colors = ShelfdColors.of(ctx);
      return SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(AppTokens.space24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Row(
                children: [
                  Icon(Icons.settings_outlined, size: 24, color: colors.ink),
                  const SizedBox(width: AppTokens.space12),
                  Text(
                    'Settings',
                    style: AppTypography.titleSerif(
                      fontSize: 20,
                      color: colors.ink,
                    ),
                  ),
                  const Spacer(),
                  IconButton(
                    icon: Icon(Icons.close_rounded, color: colors.mutedInk),
                    onPressed: () => Navigator.of(ctx).pop(),
                  ),
                ],
              ),
              const SizedBox(height: AppTokens.space16),
              Divider(color: colors.subtleBorder, height: 1),
              const SizedBox(height: AppTokens.space16),
              Row(
                children: [
                  Icon(Icons.dns_outlined, size: 20, color: colors.mutedInk),
                  const SizedBox(width: AppTokens.space12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          'Homelab Server',
                          style: AppTypography.bodySans(
                            fontSize: 14,
                            fontWeight: FontWeight.w600,
                            color: colors.ink,
                          ),
                        ),
                        Text(
                          'Connected to Shelfd Homelab Daemon',
                          style: AppTypography.bodySans(
                            fontSize: 12,
                            color: colors.mutedInk,
                          ),
                        ),
                      ],
                    ),
                  ),
                  Container(
                    width: 8,
                    height: 8,
                    decoration: BoxDecoration(
                      color: colors.accentSecondary,
                      shape: BoxShape.circle,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: AppTokens.space16),
              Consumer(
                builder: (context, ref, _) {
                  final queueState = ref.watch(queueProvider);
                  final status = queueState.status;
                  if (status == null) return const SizedBox.shrink();

                  final libraryState = ref.watch(libraryProvider);
                  final resolvedBook = resolveBookTitle(
                    status.currentBook,
                    libraryState.books,
                  );
                  final effectivePercent = status.progressPercent > 0
                      ? status.progressPercent
                      : (status.totalChapters > 0 &&
                                status.pendingChapters < status.totalChapters
                            ? ((status.totalChapters - status.pendingChapters) /
                                      status.totalChapters) *
                                  100.0
                            : 0.0);
                  final progress = (effectivePercent / 100.0).clamp(
                    0.0,
                    1.0,
                  );

                  return Container(
                    padding: const EdgeInsets.all(AppTokens.space12),
                    decoration: BoxDecoration(
                      color: colors.containerLow,
                      borderRadius: BorderRadius.circular(AppTokens.radiusMd),
                      border: Border.all(color: colors.subtleBorder),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          children: [
                            Icon(
                              Icons.auto_awesome_rounded,
                              size: 16,
                              color: colors.accentPrimary,
                            ),
                            const SizedBox(width: AppTokens.space8),
                            Text(
                              'AI Passage Indexing & Vectors',
                              style: AppTypography.bodySans(
                                fontSize: 13,
                                fontWeight: FontWeight.w600,
                                color: colors.ink,
                              ),
                            ),
                            const Spacer(),
                            Text(
                              '${effectivePercent.toStringAsFixed(0)}%',
                              style: AppTypography.mono(
                                fontSize: 12,
                                fontWeight: FontWeight.w700,
                                color: colors.accentPrimary,
                              ),
                            ),
                          ],
                        ),
                        const SizedBox(height: AppTokens.space8),
                        ClipRRect(
                          borderRadius: BorderRadius.circular(2),
                          child: LinearProgressIndicator(
                            value: progress,
                            minHeight: 4,
                            backgroundColor: colors.container,
                            valueColor: AlwaysStoppedAnimation<Color>(
                              colors.accentPrimary,
                            ),
                          ),
                        ),
                        const SizedBox(height: AppTokens.space8),
                        Text(
                          '${status.indexedChapters} of ${status.totalChapters} chapters processed (${status.pendingChapters} pending)',
                          style: AppTypography.bodySans(
                            fontSize: 12,
                            color: colors.mutedInk,
                          ),
                        ),
                        if (resolvedBook != null &&
                            resolvedBook.isNotEmpty) ...[
                          const SizedBox(height: 4),
                          Text(
                            'Now processing: $resolvedBook',
                            style: AppTypography.bodySans(
                              fontSize: 11,
                              fontWeight: FontWeight.w500,
                              color: colors.ink,
                            ),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ],
                      ],
                    ),
                  );
                },
              ),
              const SizedBox(height: AppTokens.space24),
              OutlinedButton.icon(
                style: OutlinedButton.styleFrom(
                  foregroundColor: colors.ink,
                  side: BorderSide(color: colors.border),
                  padding: const EdgeInsets.symmetric(vertical: 12),
                ),
                icon: const Icon(Icons.cloud_upload_outlined, size: 18),
                label: const Text('Uploads & Staged Queue'),
                onPressed: () {
                  Navigator.of(ctx).pop();
                  GoRouter.of(context).go('/uploads');
                },
              ),
              const SizedBox(height: AppTokens.space12),
              OutlinedButton.icon(
                style: OutlinedButton.styleFrom(
                  foregroundColor: colors.ink,
                  side: BorderSide(color: colors.border),
                  padding: const EdgeInsets.symmetric(vertical: 12),
                ),
                icon: const Icon(Icons.handyman_outlined, size: 18),
                label: const Text('Library Utilities & Merge'),
                onPressed: () {
                  Navigator.of(ctx).pop();
                  GoRouter.of(context).go('/utilities');
                },
              ),
              const SizedBox(height: AppTokens.space12),
              OutlinedButton.icon(
                style: OutlinedButton.styleFrom(
                  foregroundColor: colors.ink,
                  side: BorderSide(color: colors.border),
                  padding: const EdgeInsets.symmetric(vertical: 12),
                ),
                icon: const Icon(Icons.lock_reset_outlined, size: 18),
                label: const Text('Change Password & Account Settings'),
                onPressed: () {
                  Navigator.of(ctx).pop();
                  GoRouter.of(context).go('/settings');
                },
              ),
              const SizedBox(height: AppTokens.space12),
              OutlinedButton.icon(
                style: OutlinedButton.styleFrom(
                  foregroundColor: colors.errorText,
                  side: BorderSide(color: colors.errorBorder),
                  padding: const EdgeInsets.symmetric(vertical: 12),
                ),
                icon: const Icon(Icons.logout_rounded, size: 18),
                label: const Text('Disconnect / Sign Out'),
                onPressed: () async {
                  Navigator.of(ctx).pop();
                  await onLogout?.call();
                },
              ),
            ],
          ),
        ),
      );
    },
  );
}
