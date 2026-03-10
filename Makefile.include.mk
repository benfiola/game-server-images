define tool-from-apt
install-tools: install-tools__$(1)
.PHONY: install-tools__$(1)
install-tools__$(1): $$(bin)/$(1)
$$(bin)/$(1): | $$(bin)
	# update package index
	apt -y update
	# install $(2)
	DEBIAN_FRONTEND=noninteractive apt -y install $(2)
	# maybe symlink $(1)
	SRC="$$$$(which $(1))" DST="$$(bin)/$(1)" && if [ "$$$${SRC}" != "$$$${DST}" ]; then ln -fs "$$$${SRC}" "$$$${DST}"; fi;
endef

define tool-from-tar-gz
install-tools: install-tools__$(1)
.PHONY: install-tools__$(1)
install-tools__$(1): $$(bin)/$(1)
$$(bin)/$(1): $$(bin)/bsdtar $$(bin)/curl | $$(bin)
	# clean temp paths
	rm -rf $$(bin)/.extract $$(bin)/.archive.tar.gz && mkdir -p $$(bin)/.extract
	# download $(1) archive
	curl -o $$(bin)/.archive.tar.gz -fsSL $(2)
	# extract $(1)
	bsdtar xvzf $$(bin)/.archive.tar.gz --strip-components $(3) -C $$(bin)/.extract
	# move $(1)
	mv $$(bin)/.extract/$(1) $$(bin)/$(1)
	# clean temp paths
	rm -rf $$(bin)/.extract $$(bin)/.archive.tar.gz 
endef

